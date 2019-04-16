// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
)

// cutoffTime represents the time of a banking day when all ACH files need to be uploaded in order
// to be processed for that day. Files which miss the cutoff time won't be processed until the next day.
//
// TODO(adam): How to handle multiple cutoffTime's for Same Day ACH?
type cutoffTime struct {
	routingNumber string
	cutoff        int            // 24-hour time value (0000 to 2400)
	loc           *time.Location // timezone cutoff is in (usually America/New_York)
}

// fileTransferController is a controller which is responsible for periodic sync'ing of ACH files
// with their remote SFTP destination. The ACH network operates on uploading and downloding files
// from hosts during the business day.
type fileTransferController struct {
	rootDir   string
	batchSize int

	interval    time.Duration
	cutoffTimes []*cutoffTime

	sftpConfigs         []*sftpConfig
	fileTransferConfigs []*fileTransferConfig

	ach *achclient.ACH

	logger log.Logger
}

// newFileTransferController returns a fileTransferController which is responsible for uploading ACH files
// to their SFTP host for processing.
//
// To change the refresh duration set ACH_FILE_TRANSFER_INTERVAL with a Go time.Duration value. (i.e. 10m for 10 minutes)
func newFileTransferController(logger log.Logger, dir string, repo fileTransferRepository) (*fileTransferController, error) {
	if _, err := os.Stat(dir); dir == "" || err != nil {
		return nil, fmt.Errorf("file-transfer-controller: problem with storage directory %q: %v", dir, err)
	}

	interval, err := time.ParseDuration(os.Getenv("ACH_FILE_TRANSFER_INTERVAL"))
	if err != nil {
		interval = 10 * time.Minute
	}
	batchSize := 100
	if v := os.Getenv("ACH_FILE_BATCH_SIZE"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			batchSize = n
		}
	}
	logger.Log("file-transfer-controller", fmt.Sprintf("starting ACH file transfer controller: interval=%v batches=%d", interval, batchSize))

	cutoffTimes, err := repo.getCutoffTimes()
	if err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error reading cutoffTimes: %v", err)
	}
	sftpConfigs, err := repo.getSFTPConfigs()
	if err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error reading sftpConfigs: %v", err)
	}
	fileTransferConfigs, err := repo.getFileTransferConfigs()
	if err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error reading sftpConfigs: %v", err)
	}
	rootDir, err := filepath.Abs(dir)
	if err != nil || strings.Contains(dir, "..") {
		return nil, fmt.Errorf("file-transfer-controller: invalid directory %s: %v", dir, err)
	}
	if err := os.MkdirAll(rootDir, 0777); err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error creating %s: %v", rootDir, err)
	}

	return &fileTransferController{
		rootDir:             rootDir,
		interval:            interval,
		batchSize:           batchSize,
		cutoffTimes:         cutoffTimes,
		sftpConfigs:         sftpConfigs,
		fileTransferConfigs: fileTransferConfigs,
		ach:                 achclient.New("", logger),
		logger:              logger,
	}, nil
}

func (c *fileTransferController) getDetails(cutoff *cutoffTime) (*sftpConfig, *fileTransferConfig) {
	var sftp *sftpConfig
	for i := range c.sftpConfigs {
		if cutoff.routingNumber == c.sftpConfigs[i].RoutingNumber {
			sftp = c.sftpConfigs[i]
			break
		}
	}
	for i := range c.fileTransferConfigs {
		if cutoff.routingNumber == c.fileTransferConfigs[i].RoutingNumber {
			return sftp, c.fileTransferConfigs[i]
		}
	}
	return nil, nil
}

// startPeriodicFileOperations will block forever to periodically download incoming and returned ACH files while also merging
// and uploading ACH files to their remote SFTP server.
//
// Uploads will be completed before their cutoff time which is set for a given ABA routing number.
//
// TODO(adam): We should have a channel that is cutoffTime aware (to fire at N minutes before cutoff - to ship off every (merged) file possible)
func (c *fileTransferController) startPeriodicFileOperations(ctx context.Context, depRepo depositoryRepository, transferRepo transferRepository) {
	tick := time.NewTicker(c.interval)
	defer tick.Stop()

	// Grab shared transfer cursor for new transfers to merge into local files
	transferCursor := transferRepo.getTransferCursor(c.batchSize, depRepo)

	for {
		select {
		case <-tick.C:
			c.logger.Log("file-transfer-controller", "Starting periodic file operations")
			var wg sync.WaitGroup
			errs := make(chan error, 10)

			// For all routing numbers grab their inbound and return files
			wg.Add(1)
			go func() {
				if err := c.downloadAndProcessIncomingFiles(); err != nil {
					errs <- fmt.Errorf("downloadAndProcessIncomingFiles: %v", err)
				}
				wg.Done()
			}()

			// Grab transfers, merge them into files, and upload any which are complete.
			wg.Add(1)
			go func() {
				if err := c.mergeAndUploadFiles(transferCursor, transferRepo); err != nil {
					errs <- fmt.Errorf("mergeAndUploadFiles: %v", err)
				}
				wg.Done()
			}()

			// Wait for all operations to complete
			wg.Wait()
			errs <- nil // send so channel read doesn't block
			if err := <-errs; err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("ERROR: periodic file operation"), "error", err)
			} else {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("files sync'd, waiting %v", c.interval))
			}

		case <-ctx.Done():
			c.logger.Log("file-transfer-controller", "Shutting down due to context.Done()")
			return
		}
	}
}

// downloadAndProcessIncomingFiles will
func (c *fileTransferController) downloadAndProcessIncomingFiles() error {
	dir, err := ioutil.TempDir(c.rootDir, "downloaded")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	for i := range c.cutoffTimes {
		sftpConf, fileTransferConf := c.getDetails(c.cutoffTimes[i])
		if sftpConf == nil {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("missing sftp config for %s", c.cutoffTimes[i].routingNumber))
			continue
		}
		if fileTransferConf == nil {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("missing file transfer config for %s", c.cutoffTimes[i].routingNumber))
			continue
		}
		if err := c.downloadAllFiles(dir, sftpConf, fileTransferConf); err != nil {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("error downloading files into %s", dir), "error", err)
			continue
		}

		// Read and process inbound and returned files
		if err := c.processInboundFiles(filepath.Join(dir, fileTransferConf.InboundPath)); err != nil {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("problem reading inbound files in %s", dir), "error", err)
			continue
		}
		if err := c.processReturnFiles(filepath.Join(dir, fileTransferConf.ReturnPath)); err != nil {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("problem reading return files in %s", dir), "error", err)
			continue
		}
	}

	return nil
}

// downloadAllFiles will setup directories for each routing number and initiate downloading and writing the files to sub-directories.
func (c *fileTransferController) downloadAllFiles(dir string, sftpConf *sftpConfig, fileTransferConf *fileTransferConfig) error {
	agent, err := newFileTransferAgent(sftpConf, fileTransferConf)
	if err != nil {
		return fmt.Errorf("file-transfer-controller: problem with %s file transfer agent init: %v", sftpConf.RoutingNumber, err)
	}
	defer agent.close()

	// Setup file downloads
	if err := c.saveRemoteFiles(agent, dir); err != nil {
		c.logger.Log("file-transfer-controller", fmt.Sprintf("ERROR downloading files (ABA: %s)", sftpConf.RoutingNumber), "error", err)
	}
	return nil
}

func (c *fileTransferController) processInboundFiles(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if (err != nil && err != filepath.SkipDir) || info.IsDir() {
			return nil // Ignore SkipDir and directories
		}

		file, err := parseACHFilepath(path)
		if err != nil {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("problem parsing inbound file %s", path), "error", err)
			return nil
		}
		c.logger.Log("file-transfer-controller", fmt.Sprintf("processing inbound file %s from %s (%s)", info.Name(), file.Header.ImmediateOriginName, file.Header.ImmediateOrigin))

		// TODO(adam): What else to do with inbound files?

		return nil
	})
}

func (c *fileTransferController) processReturnFiles(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if (err != nil && err != filepath.SkipDir) || info.IsDir() {
			return nil // Ignore SkipDir and directories
		}

		file, err := parseACHFilepath(path)
		if err != nil {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("problem parsing return file %s", path), "error", err)
			return nil
		}
		c.logger.Log("file-transfer-controller", fmt.Sprintf("processing return file %s from %s (%s)", info.Name(), file.Header.ImmediateOriginName, file.Header.ImmediateOrigin))

		// TODO(adam): What else to do with return files?

		return nil
	})
}

// writeFiles will create files in dir for each file object provided
// The contents of each file struct will always be closed.
func (c *fileTransferController) writeFiles(files []file, dir string) error {
	var errordFilenames []string

	os.Mkdir(dir, 0777) // ignore errors
	for i := range files {
		f, err := os.Create(filepath.Join(dir, files[i].filename))
		if err != nil {
			errordFilenames = append(errordFilenames, files[i].filename)
			continue
		}
		if _, err = io.Copy(f, files[i].contents); err != nil {
			errordFilenames = append(errordFilenames, files[i].filename)
			continue
		}
		f.Sync()
		f.Close()
		files[i].contents.Close()
	}
	if len(errordFilenames) != 0 {
		return fmt.Errorf("writeFiles problem on: %s", strings.Join(errordFilenames, ", "))
	}
	return nil
}

// saveRemoteFiles will write all inbound and return ACH files for a given routing number to the specified directory
func (c *fileTransferController) saveRemoteFiles(agent fileTransferAgent, dir string) error {
	errs := make(chan error, 10)
	var wg sync.WaitGroup

	// Download and save inbound files
	wg.Add(1)
	go func() {
		defer wg.Done()
		files, err := agent.getInboundFiles()
		if err != nil {
			errs <- err
			return
		}
		if err := c.writeFiles(files, filepath.Join(dir, agent.inboundPath())); err != nil {
			errs <- err
			return
		}
		for i := range files {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("ACH: copied down inbound file %s", files[i].filename))

			// Delete inbound file from SFTP server
			if err := agent.delete(filepath.Join(agent.inboundPath(), files[i].filename)); err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("ACH: problem deleting inbound file %s", files[i].filename), "error", err)
			}
		}
	}()

	// Download and save returned files
	wg.Add(1)
	go func() {
		defer wg.Done()
		files, err := agent.getReturnFiles()
		if err != nil {
			errs <- err
			return
		}
		if err := c.writeFiles(files, filepath.Join(dir, agent.returnPath())); err != nil {
			errs <- err
			return
		}
		for i := range files {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("ACH: copied down return file %s", files[i].filename))

			// Delete return file from SFTP server
			if err := agent.delete(filepath.Join(agent.returnPath(), files[i].filename)); err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("ACH: problem deleting return file %s", files[i].filename), "error", err)
			}
		}
	}()

	wg.Wait()
	errs <- nil // send something incase no errors were encountered (so the channel read doesn't block)
	if err := <-errs; err != nil {
		return err
	}
	return nil
}

// loadIncomingFile will retrieve a transfer's ACH file contents and parse into an ach.File object
func (c *fileTransferController) loadIncomingFile(xfer *groupableTransfer, transferRepo transferRepository) (*ach.File, error) {
	fileId, err := transferRepo.getFileIdForTransfer(xfer.ID, xfer.userId)
	if err != nil || fileId == "" {
		return nil, err
	}
	buf, err := c.ach.GetFileContents(fileId) // read from our ACH service
	if err != nil {
		return nil, err
	}
	file, err := parseACHFile(buf)
	if err != nil {
		return nil, err
	}
	c.logger.Log("file-transfer-controller", fmt.Sprintf("merging: parsed ACH file %s for transfer %s", fileId, xfer.ID))
	return file, nil
}

// mergeTransfer will attempt to add the Batches from `file` into our mergableFile. If mergableFile exceeds ACH
// file size/length limitations then a new file will be created and the old returned for uplaod.
func (c *fileTransferController) mergeTransfer(file *ach.File, mergableFile *achFile) (*achFile, error) {
	for i := range file.Batches {
		batchExistsInMerged := false
		for j := range mergableFile.Batches {
			if file.Batches[i].Equal(mergableFile.Batches[j]) {
				batchExistsInMerged = true
			}
		}
		// Add batch into merged file
		if !batchExistsInMerged {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("adding batch %d to merged file %s", file.Batches[i].GetHeader().BatchNumber, mergableFile.filepath))

			// Add Batch, but if we surpass LoC limit then create a new file
			mergableFile.AddBatch(file.Batches[i])
			if err := mergableFile.Create(); err != nil {
				return nil, fmt.Errorf("mergable file %s failed to build: %v", mergableFile.filepath, err)
			}
			lines := mergableFile.lineCount()
			if lines == 0 {
				// indicates an error
				return nil, fmt.Errorf("mergable file %s has no lineCount", mergableFile.filepath)
			}
			if lines > 10000 {
				mergableFile.removeBatch(file.Batches[i])
				if err := mergableFile.Create(); err != nil {
					c.logger.Log("file-transfer-controller", fmt.Sprintf("problem with mergable file %s Create", mergableFile.filepath), "error", err)
					continue
				}
				if err := mergableFile.write(); err != nil {
					c.logger.Log("file-transfer-controller", fmt.Sprintf("problem flushing mergable file %s", mergableFile.filepath), "error", err)
					continue
				}

				// trim off batches we added to current mergableFile
				file.Batches = file.Batches[i:]

				// create a new mergableFile
				dir, filename := filepath.Split(mergableFile.filepath)
				filename = achFilename(file.Header.ImmediateDestination, achFilenameSeq(filename)+1)
				newMergableFile := &achFile{
					File:     file,
					filepath: filepath.Join(dir, filename),
				}
				if err := newMergableFile.Create(); err != nil {
					c.logger.Log("file-transfer-controller", fmt.Sprintf("problem with mergable file %s Create", newMergableFile.filepath), "error", err)
					continue
				}
				if err := newMergableFile.write(); err != nil {
					return nil, fmt.Errorf("problem writing mergable file %s: %v", newMergableFile.filepath, err)
				}
				return mergableFile, nil
			}
			// Call this write after we go through the == 0 check (to hope and avoid zero'ing out the file)
			if err := mergableFile.write(); err != nil {
				return nil, fmt.Errorf("problem writing mergable file %s: %v", mergableFile.filepath, err)
			}
		}
	}
	return nil, nil
}

// mergeAndUploadFiles will retrieve all Transfer objects written to paygate's database but have not yet been added
// to a file for upload to a Fed server. Any files which are ready to be upload will be uploaded, their transfer status
// updated and local copy deleted.
func (c *fileTransferController) mergeAndUploadFiles(cur *transferCursor, transferRepo transferRepository) error {
	// TODO(adam): create "mergeId" (base.ID()) for logs from a single mergeAndUploadFiles call

	// Our "merged" directory can exist from a previous run since we want to merge as many Transfer objects (ACH files) into a file as possible.
	//
	// FI's pay for each file that's uploaded, so it's important to merge and consolidate files to reduce their cost. ACH files have a maximum
	// of 10k lines before needing to be split up.
	mergedDir := filepath.Join(c.rootDir, "merged")
	os.Mkdir(mergedDir, 0777) // ensure dir is created
	c.logger.Log("file-transfer-controller", "Starting file merge and upload operations")

	errCount := 0
	for {
		groupedTransfers, err := groupTransfers(cur.Next())
		if err != nil {
			if errCount > 3 {
				return fmt.Errorf("mergeAndUploadFiles: to many errors (retries=%d): %v", errCount, err)
			}
			errCount++
			continue
		}
		if len(groupedTransfers) == 0 {
			break
		}

		var filesToUpload []*achFile

		// Group transfers by ABA and add to mergable files
		for i := range groupedTransfers {
			for j := range groupedTransfers[i] {
				if fileToUpload := c.mergeGroupableTransfer(mergedDir, groupedTransfers[i][j], transferRepo); fileToUpload != nil {
					filesToUpload = append(filesToUpload, fileToUpload)
				}
			}
		}

		// TODO(adam): We should scan for mergable files to also upload (in the event paygate crashed)

		// Upload files that are full
		// TODO(adam): also should check for cutoffTime here (and upload if we're close to cutoff)
		for i := range filesToUpload {
			for j := range c.cutoffTimes {
				if filesToUpload[i].Header.ImmediateDestination == c.cutoffTimes[j].routingNumber {
					if err := c.maybeUploadFile(filesToUpload[i], c.cutoffTimes[j]); err != nil {
						c.logger.Log("file-transfer-controller", fmt.Sprintf("problem uploading %s", filesToUpload[i].filepath), "error", err.Error())
					}
				}
			}
		}
	}

	// the other thing that does is that if you get over 10K lines you will need to increment the file header for the second
	// file of that cutoff. Which you probably don’t want to figure out in the last three minutes

	// TODO(adam): after uploading a file update all transfers with ?filename?, batch #, upload date / and success

	// We can only upload files once then after paygate relaunches it needs to scan transfers
	// that are in files (transfer row has batch #), but aren't uploaded
	// ^ those files might need re-merged/built locally and uploaded

	// uploads can be triggered and block the rest of the controller (they need to delete files and update the db)
	//  - in the event of a successful upload, but bad DB write we need to not re-upload that file (or the transfers)

	// keep an inmem checksum for each merged file? Keep the fileIds for each merged file inmem? to skip re-reading the merged files for each new transfer?
	// or maybe keep a tracking file of each? idk.

	// read transfers for current day, merge into files in scratch dir, after each batch sftp (with retries) files (optional: override sftp destination from Fed routing table and cutoff logic)
	// keep doing ^ and clear files after last cutoff of the day? -- wait, how do we sync between sftp server and ours?
	// pause at last cutoff for 1hr?

	// for each ABA get inbound and return files for parsing
	// can update transfer status, send alerts?

	// After we've downloaded and merged files let's upload any that need to be uploaded
	// (this should be accumulated somehow above)

	return nil
}

// mergeGroupableTransfer will inspect a Transfer, load the backing ACH file and attempt to merge that transfer into an existing merge file for upload.
func (c *fileTransferController) mergeGroupableTransfer(mergedDir string, xfer *groupableTransfer, transferRepo transferRepository) *achFile {
	file, err := c.loadIncomingFile(xfer, transferRepo)
	if err != nil {
		c.logger.Log("file-transfer-controller", fmt.Sprintf("problem loading ACH file conents for transfer %s", xfer.ID), "error", err)
		return nil
	}
	// Find (or create) a mergable file for this transfer's destination
	mergableFile, err := grabLatestMergedACHFile(xfer.destination, file, mergedDir)
	if err != nil {
		c.logger.Log("file-transfer-controller", fmt.Sprintf("unable to find mergable file for transfer %s", xfer.ID), "error", err)
		return nil
	}
	// Merge our transfer's file into mergableFile
	// TODO(adam): need to read batch info off the transaction and dedup against ACH file to not duplicate Batches
	fileToUpload, err := c.mergeTransfer(file, mergableFile)
	if err != nil {
		c.logger.Log("file-transfer-controller", fmt.Sprintf("merging: %v", err))
		return nil
	}
	// Assume the transfer was merged into mergableFile and so we can update its DB record.
	if err := transferRepo.markTransferAsMerged(xfer.ID, filepath.Base(mergableFile.filepath)); err != nil {
		c.logger.Log("file-transfer-controller", fmt.Sprintf("BAD ERROR - unable to mark transfer %s as merged", xfer.ID))
		// TODO(adam): This error is bad because we could end up merging the transfer into multiple files (i.e. duplicate it)
		return nil
	}
	// TODO(adam): We should check the cutoffTime against time.Now() and determine if to force the mergableFile.filepath to upload
	if fileToUpload != nil { // this is only set if existing mergableFile surpasses ACH file line limit
		c.logger.Log("file-transfer-controller",
			fmt.Sprintf("merging: scheduling %s for upload ABA:%s", fileToUpload.filepath, fileToUpload.File.Header.ImmediateDestination))
		return fileToUpload
	}
	return nil
}

// maybeUploadFile will grab the needed configs and upload an given file to the SFTP server for cutoffTime's routingNumber
func (c *fileTransferController) maybeUploadFile(fileToUpload *achFile, cutoffTime *cutoffTime) error {
	// Grab configs for setting up SFTP uploader
	sftpConf, fileTransferConf := c.getDetails(cutoffTime)
	if sftpConf == nil {
		return fmt.Errorf("missing sftp config for %s", cutoffTime.routingNumber)
	}
	if fileTransferConf == nil {
		return fmt.Errorf("missing file transfer config for %s", cutoffTime.routingNumber)
	}

	agent, err := newFileTransferAgent(sftpConf, fileTransferConf)
	if err != nil {
		return fmt.Errorf("problem creating fileTransferAgent for %s: %v", sftpConf.RoutingNumber, err)
	}
	defer agent.close()

	// TODO(adam): after upload delete the merged file (local delete)

	c.logger.Log("file-transfer-controller", fmt.Sprintf("uploading %s for routing number %s", fileToUpload.filepath, cutoffTime.routingNumber))

	return c.uploadFile(agent, fileToUpload)
}

func (c *fileTransferController) uploadFile(agent fileTransferAgent, f *achFile) error {
	fd, err := os.Open(f.filepath)
	if err != nil {
		return fmt.Errorf("problem opening %s for upload: %v", f.filepath, err)
	}
	defer fd.Close()

	if err := agent.uploadFile(file{filename: filepath.Base(f.filepath), contents: fd}); err != nil {
		return fmt.Errorf("problem uploading %s: %v", f.filepath, err)
	}
	c.logger.Log("file-transfer-controller", fmt.Sprintf("merged: uploaded file %s", f.filepath))
	return nil
}

// achFilename returns a filename for a given ACH file
//
// yyyy = Year of file creation
// MM = Month of file creation
// dd = Day of file creation
// RTN . . . = 9-digit Routing Transit Number of the bank (ODFI or RDFI) (example: 301234567)
// X = file sequence of the day, i.e., 1, 2, 3
//
// 20181222-301234567-1.ach
func achFilename(routingNumber string, seq int) string {
	return fmt.Sprintf("%s-%s-%d.ach", time.Now().Format("20060102"), routingNumber, seq)
}

// achFilenameSeq returns the sequence number from a given achFilename
// A sequence number of 0 indicates an error
func achFilenameSeq(filename string) int {
	parts := strings.Split(filename, "-")
	if len(parts) < 3 {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSuffix(parts[2], ".ach"))
	return n
}

func parseACHFilepath(path string) (*ach.File, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	return parseACHFile(fd)
}

func parseACHFile(r io.Reader) (*ach.File, error) {
	file, err := ach.NewReader(r).Read()
	if err != nil {
		return nil, err
	}
	return &file, nil
}

type achFile struct {
	*ach.File

	filepath string
}

// removeBatch will delete an ach.Batcher from the underlying ach.File
func (f *achFile) removeBatch(batch ach.Batcher) {
	// TODO(adam): handle NOC and Returns
	for i := 0; i < len(f.File.Batches); i++ {
		if batch.Equal(f.File.Batches[i]) {
			f.File.Batches = append(f.File.Batches[:i], f.File.Batches[i+1:]...) // remove batch
			i--
		}
	}
}

// lineCount tabulates the line count of the underlying ach.File
func (f *achFile) lineCount() int {
	var buf bytes.Buffer
	if err := ach.NewWriter(&buf).Write(f.File); err != nil {
		return 0
	}
	lines := 0
	s := bufio.NewScanner(&buf)
	for s.Scan() {
		if v := s.Text(); v != "" {
			lines++
		}
	}
	return lines
}

// write will overwrite f.filepath with the ach.File contents underlying achFile.
func (f *achFile) write() error {
	fd, err := os.Create(f.filepath)
	if err != nil {
		return err
	}
	if err := ach.NewWriter(fd).Write(f.File); err != nil {
		return err
	}
	if err := fd.Sync(); err != nil {
		return err
	}
	return fd.Close()
}

// grabLatestMergedACHFile will scan dir for the latest file which fits achFilename's pattern
// for the provided routingNumber.
//
// grabLatestMergedACHFile will rollover files if they're at or beyond the 10k line limit
func grabLatestMergedACHFile(destinationRoutingNumber string, incoming *ach.File, dir string) (*achFile, error) {
	matches, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("*-%s-*", destinationRoutingNumber)))
	if err != nil {
		return nil, err
	}

	// Create a new mergable file if nothing was found (i.e. new routing number)
	if len(matches) == 0 {
		// Reset FileCreation date/time
		now := time.Now()
		incoming.Header.FileCreationDate = now.Format("060102") // YYMMDD
		incoming.Header.FileCreationTime = now.Format("1504")   // HHMM

		mergableFile := &achFile{
			File:     incoming,
			filepath: filepath.Join(dir, achFilename(destinationRoutingNumber, 1)),
		}
		// flush new file to disk
		if err := mergableFile.Create(); err != nil {
			return mergableFile, err
		}
		if err := mergableFile.write(); err != nil {
			return mergableFile, err
		}
		return mergableFile, nil
	}

	// Find the latest file (by sequence number)
	sort.Strings(matches) // ascending sorting
	file, err := parseACHFilepath(matches[len(matches)-1])
	if err != nil {
		return nil, err
	}
	return &achFile{
		File:     file,
		filepath: matches[len(matches)-1],
	}, nil
}

func groupTransfers(xfers []*groupableTransfer, err error) ([][]*groupableTransfer, error) {
	if err != nil {
		return nil, err
	}
	var out [][]*groupableTransfer
	for i := range xfers {
		inserted := false
		for j := range out {
			if xfers[i].destination == out[j][0].destination {
				inserted = true
				out[j] = append(out[j], xfers[i])
			}
		}
		if !inserted {
			out = append(out, []*groupableTransfer{xfers[i]})
		}
	}
	return out, nil
}

// notes
// Samy Day ACH
//  - need to generate a separate file that also will cary a fee and have a transaction limit of $25k
//  - "You have Forward and Return Items to deal with which are two different ACH actions that PayGate will need to deal with. If we are making a forward, we originated the payment, then we run a job that checks for any new transactions. For returns, which are after the forward time, we ALWAYS check to see if there are any new files. This allows us to accept same day ach even if the bank doesn’t originate it. All of our origination logic needs to check the BatchHeader to see if the transaction was selected for Same Day ACH. The following times are probably critical to add to the configuration file."

// All of our origination logic needs to check the BatchHeader to see if the transaction was selected for Same Day ACH.
// https://www.frbservices.org/assets/financial-services/ach/091517-same-day-schedule.pdf

// Wade:
// Then you have large banks that have contracts with all of them. Frequently a larger bank will at least have eastern and western to offer a larger window of time in money movement.
// For a little background someone like Bank of American basically sorts payments and optimizes them for which fed they will be sent to for inceasing speed and decreasing cost
//
// But little banks just send it on to whomever they have a contract with
// Overall our config just needs to have a time table for Forward and Returns that we can configure FI
//
// Note: remember the first two letters of a routing number tell you which fedreserve bank the state is with
// Primary
// (01–12) 	Thrift
// (+20) 	Electronic
// (+60) 	Federal Reserve Bank
// 01 	21 	61 	Boston
// 02 	22 	62 	New York
// 03 	23 	63 	Philadelphia
// 04 	24 	64 	Cleveland
// 05 	25 	65 	Richmond
// 06 	26 	66 	Atlanta
// 07 	27 	67 	Chicago
// 08 	28 	68 	St. Louis
// 09 	29 	69 	Minneapolis
// 10 	30 	70 	Kansas City
// 11 	31 	71 	Dallas
// 12 	32 	72 	San Francisco
//
// so, we can only route to ^ if we have a config for it (configs are only written to the DB if a physical contract exists)
// If the eastern bank is past the cutoff send to the western bank

type fileTransferRepository interface {
	getCutoffTimes() ([]*cutoffTime, error)
	getSFTPConfigs() ([]*sftpConfig, error)
	getFileTransferConfigs() ([]*fileTransferConfig, error)

	close() error
}

func newFileTransferRepository(db *sql.DB) fileTransferRepository {
	if db == nil {
		return &localFileTransferRepository{}
	}

	sqliteRepo := &sqliteFileTransferRepository{db}
	cutoffCount, sftpCount, fileTransferCount := sqliteRepo.getCounts()

	if (cutoffCount + sftpCount + fileTransferCount) == 0 {
		return &localFileTransferRepository{}
	}
	return sqliteRepo
}

type sqliteFileTransferRepository struct {
	// TODO(adam): admin endpoints to read and write these configs? (w/ masked passwords)
	db *sql.DB
}

func (r *sqliteFileTransferRepository) close() error {
	return r.db.Close()
}

// getCounts returns the count of cutoffTime's, sftpConfig's, and fileTransferConfig's in the sqlite database.
//
// This is used to return localFileTransferRepository if the counts are empty (so local dev "just works").
func (r *sqliteFileTransferRepository) getCounts() (int, int, int) {
	count := func(table string) int {
		query := fmt.Sprintf(`select count(*) from %s`, table)
		stmt, err := r.db.Prepare(query)
		if err != nil {
			return 0
		}
		defer stmt.Close()

		row := stmt.QueryRow()
		var n int
		row.Scan(&n)
		return n
	}
	return count("cutoff_times"), count("sftp_configs"), count("file_transfer_configs")
}

func (r *sqliteFileTransferRepository) getCutoffTimes() ([]*cutoffTime, error) {
	query := `select routing_number, cutoff, location from cutoff_times;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var times []*cutoffTime
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cutoff cutoffTime
		var loc string
		if err := rows.Scan(&cutoff.routingNumber, &cutoff.cutoff, &loc); err != nil {
			return nil, fmt.Errorf("getCutoffTimes: scan: %v", err)
		}
		if l, err := time.LoadLocation(loc); err != nil {
			return nil, fmt.Errorf("getCutoffTimes: parsing %q failed: %v", loc, err)
		} else {
			cutoff.loc = l
		}
		times = append(times, &cutoff)
	}
	return times, nil
}

func (r *sqliteFileTransferRepository) getSFTPConfigs() ([]*sftpConfig, error) {
	query := `select routing_number, hostname, username, password from sftp_configs;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var configs []*sftpConfig
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cfg sftpConfig
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.Hostname, &cfg.Username, &cfg.Password); err != nil {
			return nil, fmt.Errorf("getSFTPConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, nil
}

func (r *sqliteFileTransferRepository) getFileTransferConfigs() ([]*fileTransferConfig, error) {
	query := `select routing_number, inbound_path, outbound_path, return_path from file_transfer_configs;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var configs []*fileTransferConfig
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cfg fileTransferConfig
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.InboundPath, &cfg.OutboundPath, &cfg.ReturnPath); err != nil {
			return nil, fmt.Errorf("getFileTransferConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, nil
}

// localFileTransferRepository is a fileTransferRepository for local dev with values that match
// apitest, testdata/ftp-server/ paths, files and FTP (fsftp) defaults.
type localFileTransferRepository struct{}

func (r *localFileTransferRepository) close() error { return nil }

func (r *localFileTransferRepository) getCutoffTimes() ([]*cutoffTime, error) {
	nyc, _ := time.LoadLocation("America/New_York")
	return []*cutoffTime{
		{
			routingNumber: "121042882",
			cutoff:        1700,
			loc:           nyc,
		},
	}, nil
}

func (r *localFileTransferRepository) getSFTPConfigs() ([]*sftpConfig, error) {
	return []*sftpConfig{
		{
			RoutingNumber: "121042882",      // from 'go run ./cmd/server' in GL
			Hostname:      "localhost:2121", // below configs for moov/fsftp:v0.1.0
			Username:      "admin",
			Password:      "123456",
		},
	}, nil
}

func (r *localFileTransferRepository) getFileTransferConfigs() ([]*fileTransferConfig, error) {
	return []*fileTransferConfig{
		{
			RoutingNumber: "121042882",
			InboundPath:   "inbound/", // below configs match paygate's testdata/ftp-server/
			OutboundPath:  "outbound/",
			ReturnPath:    "returned/",
		},
	}, nil
}

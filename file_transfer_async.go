// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
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

	// mergeDirMutex guards around the c.rootDir/storage/merged directory so writes are one by one
	// instance of mergeAndUploadFiles at a time.
	mergeDirMutex sync.Mutex

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

	return &fileTransferController{
		rootDir:             rootDir,
		interval:            interval,
		batchSize:           batchSize,
		cutoffTimes:         cutoffTimes,
		sftpConfigs:         sftpConfigs,
		fileTransferConfigs: fileTransferConfigs,
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

// TODO(adam): should we have two schedulers? or another entrypoint in the below for { select { ... } }
//
// "Ideally this schedule that is a config will close accepting new transaction for a specific window and add them to the next batch."
//
//  for { time.Sleep } based (that can cancel itself)
//  cutoffTime based to ensure files are uploaded

// startPeriodicFileOperations will block forever to periodically download incoming and returned ACH files while also merging
// and uploading ACH files to their remote SFTP server.
//
// Uploads will be completed before their cutoff time which is set for a given ABA routing number.
func (c *fileTransferController) startPeriodicFileOperations(ctx context.Context, depRepo depositoryRepository, transferRepo transferRepository) {
	// TODO(adam): This ticker could/should be aware of cutoff times and dramatically drop the interval
	tick := time.NewTicker(c.interval)
	defer tick.Stop()
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
				if err := c.mergeAndUploadFiles(depRepo, transferRepo); err != nil {
					errs <- fmt.Errorf("mergeAndUploadFiles: %v", err)
				}
				wg.Done()
			}()

			// Wait for all operations to complete
			wg.Wait()
			errs <- nil // send so channel read doesn't block
			if err := <-errs; err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("ERROR: periodic file operation: %v", err))
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
	dir, err := c.downloadAllFiles()
	if err != nil {
		return err
	}
	c.logger.Log("file-transfer-controller", fmt.Sprintf("downloaded all ACH files to %s", dir))
	// TODO(adam): read directory for inbound and returned files
	return nil
}

// downloadAllFiles will setup directories for each routing number and initiate downloading and
// writing the files to sub-directories.
func (c *fileTransferController) downloadAllFiles() (string, error) {
	dir, err := ioutil.TempDir(c.rootDir, "downloaded")
	if err != nil {
		return "", err
	}

	var wg sync.WaitGroup
	for i := range c.cutoffTimes {
		sftpConf, fileTransferConf := c.getDetails(c.cutoffTimes[i])
		if sftpConf == nil || fileTransferConf == nil {
			c.logger.Log("file-transfer-controller",
				fmt.Sprintf("missing config for %s: sftpConf:%v fileTransferConf:%v",
					c.cutoffTimes[i].routingNumber, sftpConf == nil, fileTransferConf == nil))
			continue
		}

		agent, err := newFileTransferAgent(sftpConf, fileTransferConf)
		if err != nil {
			return "", fmt.Errorf("file-transfer-controller: problem with %s file transfer agent init: %v", c.cutoffTimes[i].routingNumber, err)
		}
		defer agent.close()

		// Setup file downloads
		wg.Add(1)
		go func(routingNumber string) {
			defer wg.Done()
			if err := c.saveRemoteFiles(agent, dir); err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("ERROR downloading files (ABA: %s): %v", routingNumber, err))
			} else {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("saved ACH files (ABA: %s) to %s", routingNumber, dir))
			}
		}(c.cutoffTimes[i].routingNumber)
	}
	wg.Wait()
	return dir, nil
}

// writeFiles will create files in dir for each file object provided
// The contents of each file struct will always be closed.
func (c *fileTransferController) writeFiles(files []file, dir string) error {
	cleanup := func(files []file) {
		for i := range files {
			files[i].contents.Close() // ignore errors
		}
	}
	for i := range files {
		f, err := os.Create(filepath.Join(dir, files[i].filename))
		if err != nil {
			cleanup(files[i:])
			return err
		}
		if _, err = io.Copy(f, files[i].contents); err != nil {
			cleanup(files[i:])
			return err
		}
		f.Sync()
		f.Close()
		files[i].contents.Close()
	}
	return nil
}

// saveRemoteFiles will write all inbound and return ACH files for a given routing number to the specified directory
func (c *fileTransferController) saveRemoteFiles(agent *fileTransferAgent, dir string) error {
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
		if err := c.writeFiles(files, filepath.Join(dir, agent.config.InboundPath)); err != nil {
			errs <- err
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
		if err := c.writeFiles(files, filepath.Join(dir, agent.config.ReturnPath)); err != nil {
			errs <- err
		}
	}()

	wg.Wait()
	errs <- nil // send something incase no errors were encountered (so the channel read doesn't block)
	if err := <-errs; err != nil {
		return err
	}
	return nil
}

// mergeAndUploadFiles will retrieve all Transfer objects written to paygate's database but have not yet been added
// to a file for upload to a Fed server. Any files which are ready to be upload will be uploaded, their transfer status
// updated and local copy deleted.
func (c *fileTransferController) mergeAndUploadFiles(depRepo depositoryRepository, transferRepo transferRepository) error {
	c.mergeDirMutex.Lock()
	defer c.mergeDirMutex.Unlock()

	// Our "merged" directory can exist from a previous run since we want to merge as many Transfer objects (ACH files) into a file as possible.
	//
	// FI's pay for each file that's uploaded, so it's important to merge and consolidate files to reduce their cost. ACH files have a maximum
	// of 10k lines before needing to be split up.
	mergedDir := filepath.Join(c.rootDir, "merged")
	os.Mkdir(mergedDir, 0777) // ensure dir is created

	// Grab transfer cursor for new transfers to merge into local files
	transferCursor := transferRepo.getTransferCursor(c.batchSize, depRepo)
	fmt.Printf("transferCursor: %v\n", transferCursor)

	errCount := 0
	for {
		groupedTransfers, err := groupTransfers(transferCursor.Next())
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
			// Find the mergable file and add these (one at time) until we hit 10k lines
			// Also, need to either create or update the existing file
			mergableFile, err := grabLatestMergedACHFile(groupedTransfers[i][0].destination, mergedDir)
			if err != nil {
				return err
			}

			// Grab an existing ACH file ID to parse and merge with our local file
			fileId, err := transferRepo.getFileIdForTransfer(groupedTransfers[i][0].ID, groupedTransfers[i][0].userId)
			if err != nil || fileId == "" {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("problem reading Transfer %s fileId: %v", groupedTransfers[i][0].ID, err))
				continue
			}
			// TODO(adam): need to read batch info off the transaction and dedup against ACH file to not duplicate Batches

			// Now, read from our ACH service, parse and merge with file.AddBatch(..)
			buf, err := c.ach.GetFileContents(fileId)
			if err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("problem loading ACH file %s contents: %v", fileId, err))
				continue
			}
			file, err := parseACHFile(buf)
			if err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("problem reading ACH file %s: %v", fileId, err))
				continue
			}
			for j := range file.Batches {
				fhead := file.Batches[j].GetHeader()
				fentries := file.Batches[j].GetEntries()
				if len(fentries) == 0 {
					continue // TODO(adam): log?
				}
				for k := range mergableFile.Batches {
					mhead := mergableFile.Batches[k].GetHeader()
					mentries := mergableFile.Batches[k].GetEntries()
					if len(mentries) == 0 {
						continue // TODO(adam): log?
					}
					// Check if the Batch matches what's already in the file
					// TODO(adam): Expect this to change overtime. This might not be enough to prevent duplicates, but allow multiple legit transactions.
					if fhead.StandardEntryClassCode == mhead.StandardEntryClassCode && fhead.EffectiveEntryDate == mhead.EffectiveEntryDate {
						if fentries[0].IndividualName == mentries[0].IndividualName &&
							fentries[0].Amount == mentries[0].Amount &&
							fentries[0].DiscretionaryData == mentries[0].DiscretionaryData {
							continue // match found, don't add to mergableFile
						} else {
							mergableFile.AddBatch(file.Batches[j])

							// Try building the ACH file, if it fails when we need to remove the batch and create a new file.
							// If we creat a new file then add to filesToUpload (and to be deleted)
							//
							// TODO(adam): Check for 10k lines in the file, how?
							if err := mergableFile.Create(); err != nil {
								// TOOD(adam): remove file.Batches[j], setup to upload
								filesToUpload = append(filesToUpload, mergableFile)
							}
						}
					}
				}
			}
			if err := mergableFile.Create(); err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("problem with ACH %s file.Create(): %v", fileId, err))
				continue
				// TODO(adam): need to rollback and/or split up file at this point
			}
			if err := mergableFile.write(); err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("problem writing ACH file %s: %v", fileId, err))
				continue
			}
		}

		// Upload files that are full
		// TODO(adam): also should check for cutoffTime here (and upload if we're close to cutoff)
		for i := range filesToUpload {
			for j := range c.cutoffTimes {
				if filesToUpload[i].File.Header.ImmediateDestination == c.cutoffTimes[j].routingNumber {
					// Grab configs for setting up SFTP uploader
					sftpConf, fileTransferConf := c.getDetails(c.cutoffTimes[i])
					if sftpConf == nil || fileTransferConf == nil {
						c.logger.Log("file-transfer-controller",
							fmt.Sprintf("missing config for %s: sftpConf:%v fileTransferConf:%v",
								c.cutoffTimes[i].routingNumber, sftpConf == nil, fileTransferConf == nil))
						continue
					}
					agent, err := newFileTransferAgent(sftpConf, fileTransferConf)
					if err != nil {
						// return "", fmt.Errorf("file-transfer-controller: problem with %s file transfer agent init: %v", c.cutoffTimes[i].routingNumber, err)
						continue // TODO(adam): log
					}
					fd, err := os.Open(filesToUpload[i].filepath)
					if err != nil {
						continue // TODO(adam): log
					}
					err = agent.uploadFile(file{
						filename: filesToUpload[i].filepath,
						contents: fd,
					})
					fd.Close()
					if err := agent.close(); err != nil {
						continue // TODO(adam): log
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

	// errs <- nil // send something so channel read doesn't block
	// if err := <-errs; err != nil {
	// 	c.logger.Log("file-transfer-controller", err.Error())
	// 	return err
	// }

	c.logger.Log("file-transfer-controller", fmt.Sprintf("merged (and possibly uploaded) ACH files in %s", mergedDir))
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
// for the provided routingNumber
func grabLatestMergedACHFile(routingNumber string, dir string) (*achFile, error) {
	matches, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("*-%s-*", routingNumber)))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		fmt.Println("no matches")
		return &achFile{
			File:     ach.NewFile(),
			filepath: filepath.Join(dir, achFilename(routingNumber, 1)),
		}, nil
	}

	sort.Strings(matches)

	fd, err := os.Open(matches[len(matches)-1]) // sort.Strings sorts in ascending order
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	file, err := parseACHFile(fd)
	if err != nil {
		return nil, err
	}
	return &achFile{
		File:     file,
		filepath: fd.Name(),
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
//  - need to generate a seperate file that also will cary a fee and have a transaction limit of $25k
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
}

type sqliteFileTransferRepository struct {
	db *sql.DB
}

func (r *sqliteFileTransferRepository) getCutoffTimes() ([]*cutoffTime, error) {
	return nil, nil
}

func (r *sqliteFileTransferRepository) getSFTPConfigs() ([]*sftpConfig, error) {
	return nil, nil
}

func (r *sqliteFileTransferRepository) getFileTransferConfigs() ([]*fileTransferConfig, error) {
	return nil, nil
}

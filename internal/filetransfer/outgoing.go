// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/depository/verification/microdeposit"
	"github.com/moov-io/paygate/internal/filetransfer/admin"
	"github.com/moov-io/paygate/internal/filetransfer/config"
	"github.com/moov-io/paygate/internal/filetransfer/upload"
	"github.com/moov-io/paygate/internal/transfers"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	// fileMaxLines is the maximum line count before an ACH file is uploaded
	// to its remote server. NACHA guidelines have a hard limit of 10,000 lines.
	fileMaxLines = func() int {
		if n, err := strconv.Atoi(os.Getenv("ACH_FILE_MAX_LINES")); err == nil {
			return n
		}
		return 10000
	}()

	// forcedCutoffUploadDelta is the duration before a cutoff time where an ACH file is uploaded
	// without merging into a file.
	forcedCutoffUploadDelta = func() time.Duration {
		if v := os.Getenv("FORCED_CUTOFF_UPLOAD_DELTA"); v != "" {
			if dur, _ := time.ParseDuration(v); dur > 0 {
				return dur
			}
		}
		return 5 * time.Minute
	}()

	transfersMerged = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "transfers_merged_into_ach_files",
		Help: "Counter of transfers merged into ACH files for upload",
	}, []string{"destination", "origin"})

	filesUploaded = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "ach_files_uploaded",
		Help: "Counter of ACH files uploaded",
	}, []string{"destination", "origin"})

	fileUploadError = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "ach_file_upload_errors",
		Help: "Counter of errors encountered during ACH file uploads",
	}, []string{"destination", "origin"})
)

// mergeTransfer will attempt to add the Batches from `file` into our mergableFile. If mergableFile exceeds ACH
// file size/length limitations then a new file will be created and the old returned for uplaod.
func (c *Controller) mergeTransfer(file *ach.File, mergableFile *achFile) (*achFile, error) {
	// TODO(adam): could we just call file.MergeFiles(mergableFile) ???
	if len(file.Batches) == 0 {
		return nil, errors.New("mergeTransfer: empty batches")
	}
	for i := range file.Batches {
		batchExistsInMerged := false
		for j := range mergableFile.Batches {
			if file.Batches[i].Equal(mergableFile.Batches[j]) {
				batchExistsInMerged = true
			}
		}
		// Add batch into merged file
		if !batchExistsInMerged {
			c.logger.Log("mergeTransfer", fmt.Sprintf("adding batch %d to merged file %s", file.Batches[i].GetHeader().BatchNumber, mergableFile.filepath))

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
			if lines > fileMaxLines {
				mergableFile.File.RemoveBatch(file.Batches[i])
				if err := mergableFile.Create(); err != nil {
					c.logger.Log("mergeTransfer", fmt.Sprintf("problem with mergable file %s Create", mergableFile.filepath), "error", err)
					continue
				}
				if err := mergableFile.write(); err != nil {
					c.logger.Log("mergeTransfer", fmt.Sprintf("problem flushing mergable file %s", mergableFile.filepath), "error", err)
					continue
				}

				// trim off batches we added to current mergableFile
				file.Batches = file.Batches[i:]

				// create a new mergableFile
				cfg := c.findFileTransferConfig(file.Header.ImmediateDestination)
				dir, filename := filepath.Split(mergableFile.filepath)
				filename, err := config.RenderACHFilename(cfg.FilenameTemplate(), config.FilenameData{
					RoutingNumber: file.Header.ImmediateDestination,
					N:             config.RoundSequenceNumber(config.ACHFilenameSeq(filename) + 1),
					GPG:           false,
				})
				if err != nil {
					c.logger.Log("mergeTransfer", "error building ACH filename", "error", err)
					continue
				}
				newMergableFile := &achFile{
					File:     file,
					filepath: filepath.Join(dir, filename),
				}
				if err := newMergableFile.Create(); err != nil {
					c.logger.Log("mergeTransfer", fmt.Sprintf("problem with mergable file %s Create", newMergableFile.filepath), "error", err)
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

type mergeUploadOpts struct {
	force bool
}

func (c *Controller) mergeDir() string {
	mergedDir := filepath.Join(c.rootDir, "merged")
	if _, err := os.Stat(mergedDir); os.IsNotExist(err) {
		os.MkdirAll(mergedDir, 0777) // ensure dir is created
	}
	return mergedDir
}

func (c *Controller) GetMergedFilepaths() ([]string, error) {
	return filepath.Glob(filepath.Join(c.mergeDir(), "*.ach"))
}

// mergeAndUploadFiles will retrieve all Transfer objects written to paygate's database but have not yet been added
// to a file for upload to a Fed server. Any files which are ready to be upload will be uploaded, their transfer status
// updated and local copy deleted.
func (c *Controller) mergeAndUploadFiles(transferCur *transfers.Cursor, microDepositCur *microdeposit.Cursor, req *admin.Request, opts *mergeUploadOpts) error {
	// Our "merged" directory can exist from a previous run since we want to merge as many Transfer objects (ACH files) into a file as possible.
	//
	// FI's pay for each file that's uploaded, so it's important to merge and consolidate files to reduce their cost. ACH files have a maximum
	// of 10k lines before needing to be split up.
	if req.SkipUpload {
		c.logger.Log("file-transfer-controller", "Starging ACH merge operations")
	} else {
		c.logger.Log("file-transfer-controller", "Starting file merge and upload operations")
	}

	var filesToUpload []*achFile // accumulator

	// Read the next batch of Transfers to merge and upload. Currently no marking is done on these rows to indicate they've been picked up
	// so any attempt to run multiple paygate instances will result in duplicating Transfers on the remote FI server. We do store merged_filename
	// on Transfers, but that's only after they have been merged into a file (not in the stage of "read from DB, merging into file."
	//
	// Should we mark Transfers? We need to have a code branch that sweeps all transfers to ensure we aren't missing any.
	//
	// See: https://github.com/moov-io/paygate/issues/178
	groupedTransfers, err := groupTransfers(transferCur.Next())
	if err != nil {
		return fmt.Errorf("problem grouping transfers: %v", err)
	}
	// Group transfers by ABA and add to mergable files
	for i := range groupedTransfers {
		for j := range groupedTransfers[i] {
			if fileToUpload := c.mergeGroupableTransfer(groupedTransfers[i][j]); fileToUpload != nil {
				filesToUpload = append(filesToUpload, fileToUpload)
			}
		}
	}

	// TODO(adam): What would it take to read these as Transfer objects and re-use this method's logic? This is a lot to duplicate.
	// We need to read an ACH file back into its Transfer (see: groupableTransfer), which is doable since submitMicroDeposits creates an ACH file.
	microDeposits, err := microDepositCur.Next()
	if err != nil {
		return fmt.Errorf("problem getting micro-deposits: %v", err)
	}
	// Group micro-deposits by ABA and add to mergable files
	for i := range microDeposits {
		if file := c.mergeMicroDeposit(microDeposits[i]); file != nil {
			filesToUpload = append(filesToUpload, file)
		}
	}

	// If the request asks us to only merge then skip the upload steps
	if req.SkipUpload {
		return nil
	}

	// If we're being forced to upload everything then grab all files and upload them
	dir := c.mergeDir()
	if opts.force {
		files, err := grabAllFiles(dir)
		if err != nil {
			return fmt.Errorf("problem forcing upload of all files: %v", err)
		}
		c.logger.Log("file-transfer-controller", fmt.Sprintf("found %d files to flush outbound", len(files)), "requestID", req.RequestID)
		filesToUpload = files // upload everything found
	} else {
		// Find files close to their cutoff to enqueue
		cutoffTimes, err := c.repo.GetCutoffTimes()
		if err != nil {
			return fmt.Errorf("cutoff times: %v", err)
		}
		toUpload, err := filesNearTheirCutoff(cutoffTimes, dir)
		if err != nil {
			return fmt.Errorf("problem with filesNearTheirCutoff: %v", err)
		}
		c.logger.Log("file-transfer-controller", fmt.Sprintf("found %d files near their cutoff for upload", len(toUpload)), "requestID", req.RequestID)
		filesToUpload = append(filesToUpload, toUpload...)
	}

	// Upload any merged files that are ready
	if err := c.startUpload(filesToUpload); err != nil {
		return fmt.Errorf("problem uploading ACH files: %v", err)
	}
	return nil
}

func grabAllFiles(dir string) ([]*achFile, error) {
	var out []*achFile

	matches, err := filepath.Glob(filepath.Join(dir, "*.ach"))
	if err != nil {
		return nil, err
	}

	for i := range matches {
		if file, err := parseACHFilepath(matches[i]); err != nil {
			return nil, fmt.Errorf("grabAllFiles: problem reading %s: %v", matches[i], err)
		} else {
			out = append(out, &achFile{
				File:     file,
				filepath: matches[i],
			})
		}
	}

	return out, nil
}

func filesNearTheirCutoff(cutoffTimes []*config.CutoffTime, dir string) ([]*achFile, error) {
	var filesToUpload []*achFile

	for i := range cutoffTimes {
		matches, err := filepath.Glob(filepath.Join(dir, "*.ach"))
		if err != nil {
			return nil, fmt.Errorf("dir=%s: %v", dir, err)
		}

		// If we're close to the cutoffTime then enqueue for upload
		diff := cutoffTimes[i].Diff(time.Now().In(cutoffTimes[i].Loc))

		if diff > 0*time.Second && diff <= forcedCutoffUploadDelta {
			for j := range matches {
				file, err := parseACHFilepath(matches[j])
				if err != nil {
					return nil, fmt.Errorf("matches[%d]=%s: %v", j, matches[j], err)
				}
				filesToUpload = append(filesToUpload, &achFile{
					File:     file,
					filepath: matches[j],
				})
			}
		}
	}

	return filesToUpload, nil
}

// mergeGroupableTransfer will inspect a Transfer, load the backing ACH file and attempt to merge that transfer into an existing merge file for upload.
func (c *Controller) mergeGroupableTransfer(xfer *transfers.GroupableTransfer) *achFile {
	fileId, err := c.transferRepo.GetFileIDForTransfer(xfer.ID, xfer.UserID)
	if err != nil || fileId == "" {
		return nil
	}
	var file *ach.File // TODO(adam):

	// Find (or create) a mergable file for this transfer's destination
	mergableFile, err := c.grabLatestMergedACHFile(xfer.Destination, file)
	if err != nil {
		c.logger.Log("mergeGroupableTransfer", fmt.Sprintf("unable to find mergable file for transfer %s", xfer.ID), "error", err)
		return nil
	}
	// Merge our transfer's file into mergableFile
	fileToUpload, err := c.mergeTransfer(file, mergableFile)
	if err != nil {
		c.logger.Log("mergeGroupableTransfer", fmt.Sprintf("merging: %v", err))
		return nil
	}

	transfersMerged.With("destination", file.Header.ImmediateDestination, "origin", file.Header.ImmediateOrigin).Add(1)

	// Assume the transfer was merged into mergableFile and so we can update its DB record.
	traceNumber := ""
	if len(file.Batches) > 0 && len(file.Batches[0].GetEntries()) > 0 {
		traceNumber = file.Batches[0].GetEntries()[0].TraceNumberField()
	}
	if err := c.transferRepo.MarkTransferAsMerged(xfer.ID, filepath.Base(mergableFile.filepath), traceNumber); err != nil {
		c.logger.Log("mergeGroupableTransfer", fmt.Sprintf("BAD ERROR - unable to mark transfer %s as merged: %v", xfer.ID, err))
		// TODO(adam): This error is bad because we could end up merging the transfer into multiple files (i.e. duplicate it)
		return nil
	}
	if fileToUpload != nil { // this is only set if existing mergableFile surpasses ACH file line limit
		c.logger.Log("mergeGroupableTransfer",
			fmt.Sprintf("merging: scheduling %s for upload ABA:%s", fileToUpload.filepath, fileToUpload.File.Header.ImmediateDestination))
		return fileToUpload
	}
	return nil
}

// mergeMicroDeposit will grab the ACH file for a micro-deposit and merge it into a larger ACH file for upload to the ODFI.
func (c *Controller) mergeMicroDeposit(mc microdeposit.UploadableCredit) *achFile {
	var file *ach.File // TODO(adam):

	dep, err := c.depRepo.GetUserDepository(id.Depository(mc.DepositoryID), id.User(mc.UserID))
	if dep == nil || err != nil {
		c.logger.Log("mergeMicroDeposit", fmt.Sprintf("problem reading micro-deposit depository=%s: %v", mc.DepositoryID, err))
		return nil
	}

	// Find (or create) a mergable file for this transfer's destination
	mergableFile, err := c.grabLatestMergedACHFile(dep.RoutingNumber, file)
	if err != nil {
		c.logger.Log("mergeMicroDeposit", "unable to find mergable file for micro-deposit", "userId", mc.UserID, "error", err)
		return nil
	}
	// Merge our transfer's file into mergableFile
	fileToUpload, err := c.mergeTransfer(file, mergableFile)
	if err != nil {
		c.logger.Log("mergeMicroDeposit", fmt.Sprintf("problem during micro-deposit merging: %v", err))
		return nil
	}
	// Mark the micro-deposit as merged and record in which merged file
	if err := c.microDepositRepo.MarkMicroDepositAsMerged(filepath.Base(mergableFile.filepath), mc); err != nil {
		c.logger.Log("mergeMicroDeposit", fmt.Sprintf("BAD ERROR - unable to mark micro-deposit as merged: %v", err), "userId", mc.UserID)
		// TODO(adam): This error is bad because we could end up merging the transfer into multiple files (i.e. duplicate it)
		return nil
	}
	if fileToUpload != nil { // this is only set if existing mergableFile surpasses ACH file line limit
		c.logger.Log("mergeMicroDeposit",
			fmt.Sprintf("merging: scheduling %s for upload ABA:%s", fileToUpload.filepath, fileToUpload.File.Header.ImmediateDestination))
		return fileToUpload
	}
	return nil
}

// startUpload looks for ACH files which are ready to be uploaded and matches a CutoffTime
// to them (so we can find their upload configs).
//
// After uploading a file this method renames it to avoid uploading the file multiple times.
func (c *Controller) startUpload(filesToUpload []*achFile) error {
	for i := range filesToUpload {
		file := filesToUpload[i]

		// Attempt file upload
		if err := c.maybeUploadFile(file); err != nil {
			return fmt.Errorf("problem uploading %s: %v", file.filepath, err)
		}

		// After we've uploaded mark transfer statuses, so we don't re-collect then Transfer in the next transfers.Cursor iteration
		if n, err := c.transferRepo.MarkTransfersAsProcessed(filepath.Base(file.filepath), collectTraceNumbers(file.File)); err != nil {
			return fmt.Errorf("problem marking transfers as processed for file=%s: %v", file.filepath, err)
		} else {
			c.logger.Log("transfers", fmt.Sprintf("marked %d transfers as processed for file=%s", n, file.filepath))
		}

		// rename the file so grabLatestMergedACHFile ignores it next time
		if err := os.Rename(file.filepath, file.filepath+".uploaded"); err != nil {
			// This is a bad error to run into as it means the file will likely be uploaded twice, but if
			// the underlying FS is failing what other errors would paygate run into?
			return fmt.Errorf("error renaming %s after upload: %v", file.filepath, err)
		}
	}
	return nil
}

// maybeUploadFile will grab the needed configs and upload an given file to the ODFI's server
func (c *Controller) maybeUploadFile(file *achFile) error {
	cfg := c.findFileTransferConfig(file.Header.ImmediateOrigin)
	if cfg == nil {
		return fmt.Errorf("missing file transfer config for %s", file.Header.ImmediateOrigin)
	}

	agent, err := upload.New(c.logger, c.findAgentType(cfg.RoutingNumber), cfg, c.repo)
	if err != nil {
		fh := file.File.Header
		fileUploadError.With("origin", fh.ImmediateOrigin, "destination", fh.ImmediateDestination).Add(1)
		return fmt.Errorf("problem creating fileTransferAgent for %s: %v", cfg.RoutingNumber, err)
	}
	defer agent.Close()

	c.logger.Log("maybeUploadFile", fmt.Sprintf("uploading %s for routing number %s", file.filepath, cfg.RoutingNumber))

	// TODO(adam): I think we should have a DB table for tracking file uploads (?ach_file_uploads?)
	// with the following fields: routing number, filename, timestamp.

	return c.uploadFile(agent, file)
}

func (c *Controller) uploadFile(agent upload.Agent, f *achFile) error {
	fd, err := os.Open(f.filepath)
	if err != nil {
		fileUploadError.With("origin", f.Header.ImmediateOrigin, "destination", f.Header.ImmediateDestination).Add(1)
		return fmt.Errorf("problem opening %s for upload: %v", f.filepath, err)
	}
	defer fd.Close()

	if err := agent.UploadFile(upload.File{Filename: filepath.Base(f.filepath), Contents: fd}); err != nil {
		fileUploadError.With("origin", f.Header.ImmediateOrigin, "destination", f.Header.ImmediateDestination).Add(1)
		return fmt.Errorf("problem uploading %s: %v", f.filepath, err)
	}

	c.logger.Log("uploadFile", fmt.Sprintf("merged: uploaded file %s", f.filepath))
	filesUploaded.With("origin", f.Header.ImmediateOrigin, "destination", f.Header.ImmediateDestination).Add(1)

	return nil
}

// grabLatestMergedACHFile will scan dir for the latest file which fits achFilename's pattern
// for the provided routingNumber.
//
// grabLatestMergedACHFile will rollover files if they're at or beyond the 10k line limit
// This function will ignore files that don't end with '*.ach'
//
// TODO(adam): What if we have multiple origin routing numbers? Do we need to account for this
// in the mergable file picked/returned?
func (c *Controller) grabLatestMergedACHFile(destinationRoutingNumber string, incoming *ach.File) (*achFile, error) {
	dir := c.mergeDir()
	matches, err := filepath.Glob(filepath.Join(dir, "*.ach"))
	if err != nil {
		return nil, err
	}

	// Create a new mergable file if nothing was found (i.e. new routing number)
	if len(matches) == 0 {
		// Reset FileCreation date/time
		now := time.Now()
		incoming.Header.FileCreationDate = now.Format("060102") // YYMMDD
		incoming.Header.FileCreationTime = now.Format("1504")   // HHMM

		cfg := c.findFileTransferConfig(destinationRoutingNumber)
		filename, err := config.RenderACHFilename(cfg.FilenameTemplate(), config.FilenameData{
			RoutingNumber: incoming.Header.ImmediateDestination,
			N:             "1",
		})
		if err != nil {
			return nil, err
		}
		mergableFile := &achFile{
			File:     incoming,
			filepath: filepath.Join(dir, filename),
		}

		// We need to increment the FileIDModifier in the FileHeader when creating a new file.
		mergableFile.Header.FileIDModifier = config.RoundSequenceNumber(config.ACHFilenameSeq(filepath.Base(mergableFile.filepath))) // 0-9 followed by A-Z

		// flush new file to disk
		if err := mergableFile.Create(); err != nil {
			return mergableFile, err
		}
		if err := mergableFile.write(); err != nil {
			return mergableFile, err
		}
		return mergableFile, nil
	}

	// Find the latest file (by sequence number) that matches our ImmediateDestination
	sort.Strings(matches) // ascending sorting
	for i := len(matches) - 1; i >= 0; i-- {
		// When we encounter the first file whose destination matches ours let's use that
		file, err := parseACHFilepath(matches[i])
		if err != nil {
			return nil, err
		}
		if file.Header.ImmediateDestination == incoming.Header.ImmediateDestination {
			return &achFile{
				File:     file,
				filepath: matches[i],
			}, nil
		}
	}

	// Otherwise, we had matches but found nothing so create a file.
	cfg := c.findFileTransferConfig(destinationRoutingNumber)
	filename, err := config.RenderACHFilename(cfg.FilenameTemplate(), config.FilenameData{
		RoutingNumber: incoming.Header.ImmediateDestination,
		N:             "1",
	})
	if err != nil {
		return nil, err
	}
	mergableFile := &achFile{
		File:     incoming,
		filepath: filepath.Join(dir, filename),
	}
	if err := mergableFile.Create(); err != nil {
		return mergableFile, err
	}
	if err := mergableFile.write(); err != nil {
		return mergableFile, err
	}
	return mergableFile, nil
}

// groupTransfers will return groupableTransfers grouped according to their destination RoutingNumber
func groupTransfers(xfers []*transfers.GroupableTransfer, err error) ([][]*transfers.GroupableTransfer, error) {
	if err != nil {
		return nil, err
	}
	var out [][]*transfers.GroupableTransfer
	for i := range xfers {
		inserted := false
		for j := range out {
			if xfers[i].Destination == out[j][0].Destination {
				inserted = true
				out[j] = append(out[j], xfers[i])
			}
		}
		if !inserted {
			out = append(out, []*transfers.GroupableTransfer{xfers[i]})
		}
	}
	return out, nil
}

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	transfersMerged = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "transfers_merged_into_ach_files",
		Help: "Counter of transfers merged into ACH files for upload",
	}, []string{"destination", "origin"})

	filesUploaded = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "ach_files_uploaded",
		Help: "Counter of ACH files uploaded to their destination",
	}, []string{"destination", "origin"})
)

// mergeTransfer will attempt to add the Batches from `file` into our mergableFile. If mergableFile exceeds ACH
// file size/length limitations then a new file will be created and the old returned for uplaod.
func (c *Controller) mergeTransfer(file *ach.File, mergableFile *achFile) (*achFile, error) {
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
			if lines > c.achFileLineLimit() {
				mergableFile.removeBatch(file.Batches[i])
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
				filename, err := renderACHFilename(cfg.outboundFilenameTemplate(), filenameData{
					RoutingNumber: file.Header.ImmediateDestination,
					TransferType:  "push", // TODO(adam): where does this come from? We can only fill this in when files are segmented
					N:             roundSequenceNumber(achFilenameSeq(filename) + 1),
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

// mergeAndUploadFiles will retrieve all Transfer objects written to paygate's database but have not yet been added
// to a file for upload to a Fed server. Any files which are ready to be upload will be uploaded, their transfer status
// updated and local copy deleted.
func (c *Controller) mergeAndUploadFiles(transferCur *internal.TransferCursor, microDepositCur *internal.MicroDepositCursor, transferRepo internal.TransferRepository, req *periodicFileOperationsRequest, opts *mergeUploadOpts) error {
	// Our "merged" directory can exist from a previous run since we want to merge as many Transfer objects (ACH files) into a file as possible.
	//
	// FI's pay for each file that's uploaded, so it's important to merge and consolidate files to reduce their cost. ACH files have a maximum
	// of 10k lines before needing to be split up.
	mergedDir := filepath.Join(c.rootDir, "merged")
	os.Mkdir(mergedDir, 0777) // ensure dir is created
	c.logger.Log("file-transfer-controller", "Starting file merge and upload operations")

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
			if fileToUpload := c.mergeGroupableTransfer(mergedDir, groupedTransfers[i][j], transferRepo); fileToUpload != nil {
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
		if file := c.mergeMicroDeposit(mergedDir, microDeposits[i], microDepositCur.DepRepo); file != nil {
			filesToUpload = append(filesToUpload, file)
		}
	}

	// If we're being forced to upload everything then grab all files and upload them
	if opts.force {
		files, err := grabAllFiles(mergedDir)
		if err != nil {
			return fmt.Errorf("problem forcing upload of all files: %v", err)
		}
		c.logger.Log("file-transfer-controller", fmt.Sprintf("found %d files to flush outbound", len(files)), "requestID", req.requestID)
		filesToUpload = files // upload everything found
	} else {
		// Find files close to their cutoff to enqueue
		cutoffTimes, err := c.repo.GetCutoffTimes()
		if err != nil {
			return fmt.Errorf("cutoff times: %v", err)
		}
		toUpload, err := filesNearTheirCutoff(cutoffTimes, mergedDir, c.cutoffForceThreshold())
		if err != nil {
			return fmt.Errorf("problem with filesNearTheirCutoff: %v", err)
		}
		c.logger.Log("file-transfer-controller", fmt.Sprintf("found %d files near their cutoff for upload", len(toUpload)), "requestID", req.requestID)
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

func filesNearTheirCutoff(cutoffTimes []*CutoffTime, dir string, delta time.Duration) ([]*achFile, error) {
	var filesToUpload []*achFile

	for i := range cutoffTimes {
		matches, err := filepath.Glob(filepath.Join(dir, "*.ach"))
		if err != nil {
			return nil, fmt.Errorf("dir=%s: %v", dir, err)
		}

		// If we're close to the cutoffTime then enqueue for upload
		diff := cutoffTimes[i].Diff(time.Now().In(cutoffTimes[i].Loc))

		if diff > 0*time.Second && diff <= delta {
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

// loadRemoteACHFile will retrieve a transfer's ACH file contents and parse into an ach.File object
func (c *Controller) loadRemoteACHFile(fileId string) (*ach.File, error) {
	buf, err := c.ach.GetFileContents(fileId) // read from our ACH service
	if err != nil {
		return nil, err
	}
	file, err := parseACHFile(buf)
	if err != nil {
		return nil, err
	}
	c.logger.Log("loadRemoteACHFile", fmt.Sprintf("merging: parsed ACH file %s", fileId))
	return file, nil
}

// mergeGroupableTransfer will inspect a Transfer, load the backing ACH file and attempt to merge that transfer into an existing merge file for upload.
func (c *Controller) mergeGroupableTransfer(mergedDir string, xfer *internal.GroupableTransfer, transferRepo internal.TransferRepository) *achFile {
	fileId, err := transferRepo.GetFileIDForTransfer(xfer.ID, xfer.UserID())
	if err != nil || fileId == "" {
		return nil
	}
	file, err := c.loadRemoteACHFile(fileId)
	if err != nil {
		c.logger.Log("mergeGroupableTransfer", fmt.Sprintf("problem loading ACH file conents for transfer %s", xfer.ID), "error", err)
		return nil
	}

	// Find (or create) a mergable file for this transfer's destination
	mergableFile, err := c.grabLatestMergedACHFile(xfer.Origin, file, mergedDir)
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
	if err := transferRepo.MarkTransferAsMerged(xfer.ID, filepath.Base(mergableFile.filepath), traceNumber); err != nil {
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
func (c *Controller) mergeMicroDeposit(mergedDir string, mc internal.UploadableMicroDeposit, depRepo *internal.SQLDepositoryRepo) *achFile {
	file, err := c.loadRemoteACHFile(mc.FileID)
	if err != nil {
		c.logger.Log("mergeMicroDeposit", fmt.Sprintf("error reading ACH file=%s: %v", mc.FileID, err))
		return nil
	}
	dep, err := depRepo.GetUserDepository(internal.DepositoryID(mc.DepositoryID), mc.UserID)
	if dep == nil || err != nil {
		c.logger.Log("mergeMicroDeposit", fmt.Sprintf("problem reading micro-deposit depository=%s: %v", mc.DepositoryID, err))
		return nil
	}

	// Find (or create) a mergable file for this transfer's destination
	mergableFile, err := c.grabLatestMergedACHFile(dep.RoutingNumber, file, mergedDir) // TODO(adam): is this dep.RoutingNumber the odfiAccount.RoutingNumber (our ODFI's oritin)
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
	if err := depRepo.MarkMicroDepositAsMerged(filepath.Base(mergableFile.filepath), mc); err != nil {
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

		if err := c.maybeUploadFile(file); err != nil {
			return fmt.Errorf("problem uploading %s: %v", file.filepath, err)
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
	agent, err := New(c.logger, c.findTransferType(cfg.RoutingNumber), c.config, cfg, c.repo)
	if err != nil {
		return fmt.Errorf("problem creating fileTransferAgent for %s: %v", cfg.RoutingNumber, err)
	}
	defer agent.Close()

	c.logger.Log("maybeUploadFile", fmt.Sprintf("uploading %s for routing number %s", file.filepath, cfg.RoutingNumber))

	// TODO(adam): I think we should have a DB table for tracking file uploads (?ach_file_uploads?)
	// with the following fields: routing number, filename, timestamp.

	return c.uploadFile(agent, file)
}

func (c *Controller) uploadFile(agent Agent, f *achFile) error {
	fd, err := os.Open(f.filepath)
	if err != nil {
		return fmt.Errorf("problem opening %s for upload: %v", f.filepath, err)
	}
	defer fd.Close()

	if err := agent.UploadFile(File{Filename: filepath.Base(f.filepath), Contents: fd}); err != nil {
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
func (c *Controller) grabLatestMergedACHFile(originRoutingNumber string, incoming *ach.File, dir string) (*achFile, error) { // TODO(adam): shouldn't this be the destination routing number?
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

		cfg := c.findFileTransferConfig(originRoutingNumber)
		filename, err := renderACHFilename(cfg.outboundFilenameTemplate(), filenameData{
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
		mergableFile.Header.FileIDModifier = roundSequenceNumber(achFilenameSeq(filepath.Base(mergableFile.filepath))) // 0-9 followed by A-Z

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
	cfg := c.findFileTransferConfig(originRoutingNumber)
	filename, err := renderACHFilename(cfg.outboundFilenameTemplate(), filenameData{
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

// groupTransfers will return groupableTransfers grouped according to their origin RoutingNumber
func groupTransfers(xfers []*internal.GroupableTransfer, err error) ([][]*internal.GroupableTransfer, error) {
	if err != nil {
		return nil, err
	}
	var out [][]*internal.GroupableTransfer
	for i := range xfers {
		inserted := false
		for j := range out {
			if xfers[i].Origin == out[j][0].Origin {
				inserted = true
				out[j] = append(out[j], xfers[i])
			}
		}
		if !inserted {
			out = append(out, []*internal.GroupableTransfer{xfers[i]})
		}
	}
	return out, nil
}

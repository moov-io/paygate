// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package paygate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
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
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/filetransfer"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
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
	// TODO(adam): Should we hold off uploading instead?
	forcedCutoffUploadDelta = func() time.Duration {
		if v := os.Getenv("FORCED_CUTOFF_UPLOAD_DELTA"); v != "" {
			if dur, _ := time.ParseDuration(v); dur > 0 {
				return dur
			}
		}
		return 5 * time.Minute
	}()

	// Prometheus metrics

	inboundFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "inbound_ach_files_processed",
		Help: "Counter of inbound files processed by paygate",
	}, []string{"destination", "origin"})
	returnFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "return_ach_files_processed",
		Help: "Counter of return files processed",
	}, []string{"destination", "origin"})

	transfersMerged = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "transfers_merged_into_ach_files",
		Help: "Counter of transfers merged into ACH files for upload",
	}, []string{"destination", "origin"})

	missingFileUploadConfigs = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "missing_ach_file_upload_configs",
		Help: "Counter of missing configurations for file upload - ftp, sftp, or file transfer config(s)",
	}, []string{"routing_number"})

	filesUploaded = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "ach_files_uploaded",
		Help: "Counter of ACH files uploaded to their destination",
	}, []string{"destination", "origin"})
)

// fileTransferController is a controller which is responsible for periodic sync'ing of ACH files
// with their remote FTP/SFTP destination. The ACH network operates on uploading and downloding files
// from hosts during the business day.
type fileTransferController struct {
	rootDir   string
	batchSize int

	interval    time.Duration
	cutoffTimes []*filetransfer.CutoffTime

	repo                filetransfer.Repository
	ftpConfigs          []*filetransfer.FTPConfig
	sftpConfigs         []*filetransfer.SFTPConfig
	fileTransferConfigs []*filetransfer.Config

	ach            *achclient.ACH
	accountsClient AccountsClient

	logger log.Logger
}

// NewFileTransferController returns a fileTransferController which is responsible for uploading ACH files
// to their SFTP host for processing.
//
// To change the refresh duration set ACH_FILE_TRANSFER_INTERVAL with a Go time.Duration value. (i.e. 10m for 10 minutes)
func NewFileTransferController(logger log.Logger, dir string, repo filetransfer.Repository, achClient *achclient.ACH, accountsClient AccountsClient, accountsCallsDisabled bool) (*fileTransferController, error) {
	if _, err := os.Stat(dir); dir == "" || err != nil {
		return nil, fmt.Errorf("file-transfer-controller: problem with storage directory %q: %v", dir, err)
	}

	var interval time.Duration
	if v := os.Getenv("ACH_FILE_TRANSFER_INTERVAL"); strings.EqualFold(v, "off") {
		logger.Log("file-transfer-controller", "disabling fileTransferController via config (ACH_FILE_TRANSFER_INTERVAL)")
		return nil, nil // disabled, so return nothing
	} else {
		dur, err := time.ParseDuration(v)
		if err != nil {
			interval = 10 * time.Minute
		} else {
			interval = dur
		}
	}
	batchSize := 100
	if v := os.Getenv("ACH_FILE_BATCH_SIZE"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			batchSize = n
		}
	}
	logger.Log("newFileTransferController", fmt.Sprintf("starting ACH file transfer controller: interval=%v batchSize=%d", interval, batchSize))

	cutoffTimes, err := repo.GetCutoffTimes()
	if err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error reading cutoffTimes: %v", err)
	}
	ftpConfigs, err := repo.GetFTPConfigs()
	if err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error reading ftpConfigs: %v", err)
	}
	sftpConfigs, err := repo.GetSFTPConfigs()
	if err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error reading sftpConfigs: %v", err)
	}
	fileTransferConfigs, err := repo.GetConfigs()
	if err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error reading file transfer Configs: %v", err)
	}
	rootDir, err := filepath.Abs(dir)
	if err != nil || strings.Contains(dir, "..") {
		return nil, fmt.Errorf("file-transfer-controller: invalid directory %s: %v", dir, err)
	}
	if err := os.MkdirAll(rootDir, 0777); err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error creating %s: %v", rootDir, err)
	}

	controller := &fileTransferController{
		rootDir:             rootDir,
		interval:            interval,
		batchSize:           batchSize,
		cutoffTimes:         cutoffTimes,
		repo:                repo,
		ftpConfigs:          ftpConfigs,
		sftpConfigs:         sftpConfigs,
		fileTransferConfigs: fileTransferConfigs,
		ach:                 achClient,
		logger:              logger,
	}
	if !accountsCallsDisabled {
		controller.accountsClient = accountsClient
	}
	return controller, nil
}

func (c *fileTransferController) findFileTransferConfig(cutoff *filetransfer.CutoffTime) *filetransfer.Config {
	for i := range c.fileTransferConfigs {
		if cutoff.RoutingNumber == c.fileTransferConfigs[i].RoutingNumber {
			return c.fileTransferConfigs[i]
		}
	}
	return nil
}

// findTransferType will return a string from matching the provided routingNumber against
// FTP, SFTP (and future) file transport protocols. This string needs to match filetransfer.New.
func (c *fileTransferController) findTransferType(routingNumber string) string {
	for i := range c.ftpConfigs {
		if routingNumber == c.ftpConfigs[i].RoutingNumber {
			return "ftp"
		}
	}
	for i := range c.sftpConfigs {
		if routingNumber == c.sftpConfigs[i].RoutingNumber {
			return "sftp"
		}
	}
	return "unknown"
}

// StartPeriodicFileOperations will block forever to periodically download incoming and returned ACH files while also merging
// and uploading ACH files to their remote SFTP server. forceUpload is a channel for manually triggering the "merge and upload"
// portion of this pooling loop, which is used by admin endpoints and to make testing easier.
//
// Uploads will be completed before their cutoff time which is set for a given ABA routing number.
func (c *fileTransferController) StartPeriodicFileOperations(ctx context.Context, forceUpload chan struct{}, depRepo DepositoryRepository, transferRepo transferRepository) {
	tick := time.NewTicker(c.interval)
	defer tick.Stop()

	// Grab shared transfer cursor for new transfers to merge into local files
	transferCursor := transferRepo.getTransferCursor(c.batchSize, depRepo)
	microDepositCursor := depRepo.getMicroDepositCursor(c.batchSize)

	for {
		// Setup our concurrnet waiting
		var wg sync.WaitGroup
		errs := make(chan error, 10)

		select {
		case <-forceUpload:
			c.logger.Log("StartPeriodicFileOperations", "forcing merge and upload of ACH files")
			goto uploadFiles

		case <-tick.C:
			// This is triggered by the time.Ticker (which accounts for delays) so let's download and upload files.
			c.logger.Log("StartPeriodicFileOperations", "Starting periodic file operations")
			wg.Add(1)
			go func() {
				if err := c.downloadAndProcessIncomingFiles(depRepo, transferRepo); err != nil {
					errs <- fmt.Errorf("downloadAndProcessIncomingFiles: %v", err)
				}
				wg.Done()
			}()
			goto uploadFiles

		case <-ctx.Done():
			c.logger.Log("StartPeriodicFileOperations", "Shutting down due to context.Done()")
			return
		}

	uploadFiles:
		// Grab transfers, merge them into files, and upload any which are complete.
		wg.Add(1)
		go func() {
			if err := c.mergeAndUploadFiles(transferCursor, microDepositCursor, transferRepo); err != nil {
				errs <- fmt.Errorf("mergeAndUploadFiles: %v", err)
			}
			wg.Done()
		}()

		// Wait for all operations to complete
		wg.Wait()
		errs <- nil // send so channel read doesn't block
		if err := <-errs; err != nil {
			c.logger.Log("StartPeriodicFileOperations", fmt.Sprintf("ERROR: periodic file operation"), "error", err)
		} else {
			c.logger.Log("StartPeriodicFileOperations", fmt.Sprintf("files sync'd, waiting %v", c.interval))
		}
	}
}

// downloadAndProcessIncomingFiles will take each cutoffTime initialized with the controller and retrieve all files
// on the remote server for them. After this method will call processInboundFiles and processReturnFiles on each
// downloaded file.
func (c *fileTransferController) downloadAndProcessIncomingFiles(depRepo DepositoryRepository, transferRepo transferRepository) error {
	dir, err := ioutil.TempDir(c.rootDir, "downloaded")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	for i := range c.cutoffTimes {
		fileTransferConf := c.findFileTransferConfig(c.cutoffTimes[i])
		if fileTransferConf == nil {
			c.logger.Log("downloadAndProcessIncomingFiles", fmt.Sprintf("missing file transfer config for %s", c.cutoffTimes[i].RoutingNumber))
			missingFileUploadConfigs.With("routing_number", c.cutoffTimes[i].RoutingNumber).Add(1)
			continue
		}
		if err := c.downloadAllFiles(dir, fileTransferConf); err != nil {
			c.logger.Log("downloadAndProcessIncomingFiles", fmt.Sprintf("error downloading files into %s", dir), "error", err)
			continue
		}

		// Read and process inbound and returned files
		if err := c.processInboundFiles(filepath.Join(dir, fileTransferConf.InboundPath)); err != nil {
			c.logger.Log("downloadAndProcessIncomingFiles", fmt.Sprintf("problem reading inbound files in %s", dir), "error", err)
			continue
		}
		if err := c.processReturnFiles(filepath.Join(dir, fileTransferConf.ReturnPath), depRepo, transferRepo); err != nil {
			c.logger.Log("downloadAndProcessIncomingFiles", fmt.Sprintf("problem reading return files in %s", dir), "error", err)
			continue
		}
	}

	return nil
}

// downloadAllFiles will setup directories for each routing number and initiate downloading and writing the files to sub-directories.
func (c *fileTransferController) downloadAllFiles(dir string, fileTransferConf *filetransfer.Config) error {
	agent, err := filetransfer.New(c.logger, c.findTransferType(fileTransferConf.RoutingNumber), fileTransferConf, c.repo) // TODO(adam): pass through _type
	if err != nil {
		return fmt.Errorf("downloadAllFiles: problem with %s file transfer agent init: %v", fileTransferConf.RoutingNumber, err)
	}
	defer agent.Close()

	// Setup file downloads
	if err := c.saveRemoteFiles(agent, dir); err != nil {
		c.logger.Log("downloadAllFiles", fmt.Sprintf("ERROR downloading files (ABA: %s)", fileTransferConf.RoutingNumber), "error", err)
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
			c.logger.Log("processInboundFiles", fmt.Sprintf("problem parsing inbound file %s", path), "error", err)
			return nil
		}
		c.logger.Log("file-transfer-controller", fmt.Sprintf("processing inbound file %s from %s (%s)", info.Name(), file.Header.ImmediateOriginName, file.Header.ImmediateOrigin))

		inboundFilesProcessed.With("destination", file.Header.ImmediateDestination, "origin", file.Header.ImmediateOrigin).Add(1)

		// TODO(adam): read inbound files to update a status (or process, i.e. IAT)

		return nil
	})
}

func (c *fileTransferController) processReturnFiles(dir string, depRepo DepositoryRepository, transferRepo transferRepository) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if (err != nil && err != filepath.SkipDir) || info.IsDir() {
			return nil // Ignore SkipDir and directories
		}

		file, err := parseACHFilepath(path)
		if err != nil {
			c.logger.Log("processReturnFiles", fmt.Sprintf("problem parsing return file %s", path), "error", err)
			return nil
		}
		c.logger.Log("processReturnFiles", fmt.Sprintf("processing return file %s from %s (%s)", info.Name(), file.Header.ImmediateOriginName, file.Header.ImmediateOrigin))

		returnFilesProcessed.With("destination", file.Header.ImmediateDestination, "origin", file.Header.ImmediateOrigin).Add(1)

		// Process each returned Batch and update their Transfer status
		//
		// We match the return file against transfers in our database and try to compare against fields
		// that can't change (and if they do it's clearly a different transfer).
		for i := range file.ReturnEntries {
			entries := file.ReturnEntries[i].GetEntries()
			for j := range entries {
				// Skip if the ach.Batch is invalid (for returns)
				if entries[j].Addenda99 == nil || entries[j].Addenda99.ReturnCodeField() == nil {
					c.logger.Log("processReturnFiles", "empty Addenda99 (or ReturnCode)", "traceNumber", entries[j].TraceNumber)
					continue
				}
				if err := c.processReturnEntry(file.Header, file.ReturnEntries[i].GetHeader(), entries[j], depRepo, transferRepo); err != nil {
					c.logger.Log("processReturnFiles", "error processing EntryDetail", "traceNumber", entries[j].TraceNumber, "error", err)
					continue
				}
			}
		}
		return nil
	})
}

func (c *fileTransferController) processReturnEntry(fileHeader ach.FileHeader, header *ach.BatchHeader, entry *ach.EntryDetail, depRepo DepositoryRepository, transferRepo transferRepository) error {
	effectiveEntryDate, err := time.Parse("060102", header.EffectiveEntryDate) // YYMMDD
	if err != nil {
		return fmt.Errorf("invalid EffectiveEntryDate=%q: %v", header.EffectiveEntryDate, err)
	}

	// Grab the transfer from our database
	amount, _ := NewAmountFromInt("USD", entry.Amount)
	transfer, err := transferRepo.lookupTransferFromReturn(header.StandardEntryClassCode, amount, entry.TraceNumber, effectiveEntryDate)
	if err != nil || transfer == nil || transfer.userID == "" {
		return fmt.Errorf("transfer not found: lookupTransferFromReturn: %v", err)
	}

	requestID := base.ID()
	returnCode := entry.Addenda99.ReturnCodeField()
	c.logger.Log("processReturnEntry", fmt.Sprintf("matched traceNumber=%s to transfer=%s with returnCode=%s", entry.TraceNumber, transfer.ID, returnCode), "requestID", requestID)

	// Set the ReturnCode and update the transfer's status
	if err := transferRepo.setReturnCode(transfer.ID, returnCode.Code); err != nil {
		return fmt.Errorf("problem updating ReturnCode transfer=%q: %v", transfer.ID, err)
	}
	if err := transferRepo.updateTransferStatus(transfer.ID, TransferReclaimed); err != nil {
		return fmt.Errorf("problem updating transfer=%q: %v", transfer.ID, err)
	}

	// Reverse the transaction against Accounts
	if c.accountsClient != nil && transfer.transactionID != "" {
		if err := c.accountsClient.ReverseTransaction(requestID, transfer.userID, transfer.transactionID); err != nil {
			return fmt.Errorf("problem with accounts ReverseTransaction: %v", err)
		}
	}

	// Match user Depositories to our ACH file (the user needs to have Depositories verified for this file)
	depositories, err := depRepo.getUserDepositories(transfer.userID)
	if err != nil {
		return fmt.Errorf("unable to find Depositories: %v", err)
	}
	var origDep *Depository
	var recDep *Depository
	for k := range depositories {
		if depositories[k].Status != DepositoryVerified {
			continue // We only allow Verified Depositories
		}
		if fileHeader.ImmediateOrigin == depositories[k].RoutingNumber { // TODO(adam): Should we match the originator's account number?
			origDep = depositories[k] // Originator Depository matched
		}
		if depositories[k].RoutingNumber == fileHeader.ImmediateDestination && depositories[k].AccountNumber == entry.DFIAccountNumber {
			recDep = depositories[k] // Receiver Depository matched
		}
	}
	if origDep == nil || recDep == nil {
		p := func(d *Depository) string {
			if d == nil {
				return ""
			} else {
				return string(d.ID)
			}
		}
		return fmt.Errorf("depository not found origDep=%q recDep=%q", p(origDep), p(recDep))
	}
	c.logger.Log("processReturnEntry", fmt.Sprintf("found deposiories for transfer=%s (originator=%s) (receiver=%s)", transfer.ID, origDep.ID, recDep.ID), "requestID", requestID)

	// Optionally update the Depositories for this Transfer if the return code justifies it
	if err := updateDepositoryFromReturnCode(c.logger, returnCode, origDep, recDep, depRepo); err != nil {
		return fmt.Errorf("problem with updateDepositoryFromReturnCode transfer=%q: %v", transfer.ID, err)
	}
	return nil
}

// updateDepositoryFromReturnCode will inspect the ach.ReturnCode and optionally update either the originating or receiving Depository.
// Updates are performed in cases like: death, account closure, authorization revoked, etc as specified in NACHA return codes.
func updateDepositoryFromReturnCode(logger log.Logger, code *ach.ReturnCode, origDep *Depository, destDep *Depository, depRepo DepositoryRepository) error {
	switch code.Code {
	case "R02", "R07", "R10": // "Account Closed", "Authorization Revoked by Customer", "Customer Advises Not Authorized"
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s for returnCode=%s", destDep.ID, code.Code))
		return depRepo.updateDepositoryStatus(destDep.ID, DepositoryRejected)

	case "R05": // Improper Debit to Consumer Account
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s for returnCode=%s", destDep.ID, code.Code))
		return depRepo.updateDepositoryStatus(destDep.ID, DepositoryRejected)

	case "R14", "R15": // "Representative payee deceased or unable to continue in that capacity", "Beneficiary or bank account holder"
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s and depository=%s for returnCode=%s", origDep.ID, destDep.ID, code.Code))
		if err := depRepo.updateDepositoryStatus(origDep.ID, DepositoryRejected); err != nil {
			return err
		}
		return depRepo.updateDepositoryStatus(destDep.ID, DepositoryRejected)

	case "R16": // "Bank account frozen"
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s for returnCode=%s", destDep.ID, code.Code))
		return depRepo.updateDepositoryStatus(destDep.ID, DepositoryRejected)

	case "R20": // "Non-payment bank account"
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s for returnCode=%s", destDep.ID, code.Code))
		return depRepo.updateDepositoryStatus(destDep.ID, DepositoryRejected)
	}
	return fmt.Errorf("unhandled return code: %s", code.Code)
}

// writeFiles will create files in dir for each file object provided
// The contents of each file struct will always be closed.
func (c *fileTransferController) writeFiles(files []filetransfer.File, dir string) error {
	var firstErr error
	var errordFilenames []string

	os.MkdirAll(dir, 0777) // ignore errors
	for i := range files {
		f, err := os.Create(filepath.Join(dir, files[i].Filename))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			errordFilenames = append(errordFilenames, files[i].Filename)
			continue
		}
		if _, err = io.Copy(f, files[i].Contents); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			errordFilenames = append(errordFilenames, files[i].Filename)
			continue
		}
		f.Sync()
		f.Close()
		files[i].Contents.Close()
	}
	if len(errordFilenames) != 0 {
		return fmt.Errorf("writeFiles problem on: %s: %v", strings.Join(errordFilenames, ", "), firstErr)
	}
	return nil
}

// saveRemoteFiles will write all inbound and return ACH files for a given routing number to the specified directory
func (c *fileTransferController) saveRemoteFiles(agent filetransfer.Agent, dir string) error {
	var errors []string

	// Download and save inbound files
	files, err := agent.GetInboundFiles()
	if err != nil {
		errors = append(errors, fmt.Sprintf("%T: GetInboundFiles error=%v", agent, err))
	}
	// TODO(adam): should we move this into GetInboundFiles with an LStat guard?
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, agent.InboundPath())), 0777); err != nil {
		errors = append(errors, fmt.Sprintf("%T: inbound MkdirAll error=%v", agent, err))
	}
	if err := c.writeFiles(files, filepath.Join(dir, agent.InboundPath())); err != nil {
		errors = append(errors, fmt.Sprintf("%T: inbound writeFiles error=%v", agent, err))
	}
	for i := range files {
		c.logger.Log("saveRemoteFiles", fmt.Sprintf("%T: copied down inbound file %s", agent, files[i].Filename))

		if err := agent.Delete(filepath.Join(agent.InboundPath(), files[i].Filename)); err != nil {
			errors = append(errors, fmt.Sprintf("%T: inbound Delete filename=%s error=%v", agent, files[i].Filename, err))
		}
	}

	// Download and save returned files
	files, err = agent.GetReturnFiles()
	if err != nil {
		errors = append(errors, fmt.Sprintf("%T: GetReturnFiles error=%v", agent, err))
	}
	// TODO(adam): should we move this into GetReturnFiles with an LStat guard?
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, agent.ReturnPath())), 0777); err != nil {
		errors = append(errors, fmt.Sprintf("%T: return MkdirAll error=%v", agent, err))
	}
	if err := c.writeFiles(files, filepath.Join(dir, agent.ReturnPath())); err != nil {
		errors = append(errors, fmt.Sprintf("%T: return writeFiles error=%v", agent, err))
	}
	for i := range files {
		c.logger.Log("saveRemoteFiles", fmt.Sprintf("%T: copied down return file %s", agent, files[i].Filename))

		if err := agent.Delete(filepath.Join(agent.ReturnPath(), files[i].Filename)); err != nil {
			errors = append(errors, fmt.Sprintf("%T: return Delete filename=%s error=%v", agent, files[i].Filename, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("  " + strings.Join(errors, "\n  "))
	}
	return nil
}

// loadIncomingFile will retrieve a transfer's ACH file contents and parse into an ach.File object
func (c *fileTransferController) loadIncomingFile(fileId string) (*ach.File, error) {
	buf, err := c.ach.GetFileContents(fileId) // read from our ACH service
	if err != nil {
		return nil, err
	}
	file, err := parseACHFile(buf)
	if err != nil {
		return nil, err
	}
	c.logger.Log("loadIncomingFile", fmt.Sprintf("merging: parsed ACH file %s", fileId))
	return file, nil
}

// mergeTransfer will attempt to add the Batches from `file` into our mergableFile. If mergableFile exceeds ACH
// file size/length limitations then a new file will be created and the old returned for uplaod.
func (c *fileTransferController) mergeTransfer(file *ach.File, mergableFile *achFile) (*achFile, error) {
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
				dir, filename := filepath.Split(mergableFile.filepath)
				filename = achFilename(file.Header.ImmediateDestination, achFilenameSeq(filename)+1)
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

// mergeAndUploadFiles will retrieve all Transfer objects written to paygate's database but have not yet been added
// to a file for upload to a Fed server. Any files which are ready to be upload will be uploaded, their transfer status
// updated and local copy deleted.
func (c *fileTransferController) mergeAndUploadFiles(transferCur *transferCursor, microDepositCur *microDepositCursor, transferRepo transferRepository) error {
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
		if file := c.mergeMicroDeposit(mergedDir, microDeposits[i], microDepositCur.depRepo); file != nil {
			filesToUpload = append(filesToUpload, file)
		}
	}

	// Find files close to their cutoff to enqueue
	toUpload, err := filesNearTheirCutoff(c.cutoffTimes, mergedDir)
	if err != nil {
		return fmt.Errorf("problem with filesNearTheirCutoff: %v", err)
	}
	filesToUpload = append(filesToUpload, toUpload...)

	// Upload any merged files that are ready
	if err := c.startUpload(filesToUpload); err != nil {
		return fmt.Errorf("problem uploading ACH files: %v", err)
	}
	return nil
}

func filesNearTheirCutoff(cutoffTimes []*filetransfer.CutoffTime, dir string) ([]*achFile, error) {
	var filesToUpload []*achFile

	for i := range cutoffTimes {
		pattern := filepath.Join(dir, fmt.Sprintf("*-%s-*.ach", cutoffTimes[i].RoutingNumber))
		matches, err := filepath.Glob(pattern)
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
func (c *fileTransferController) mergeGroupableTransfer(mergedDir string, xfer *groupableTransfer, transferRepo transferRepository) *achFile {
	fileId, err := transferRepo.getFileIDForTransfer(xfer.ID, xfer.userID)
	if err != nil || fileId == "" {
		return nil
	}
	file, err := c.loadIncomingFile(fileId)
	if err != nil {
		c.logger.Log("mergeGroupableTransfer", fmt.Sprintf("problem loading ACH file conents for transfer %s", xfer.ID), "error", err)
		return nil
	}

	// Find (or create) a mergable file for this transfer's destination
	mergableFile, err := grabLatestMergedACHFile(xfer.origin, file, mergedDir)
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
	if err := transferRepo.markTransferAsMerged(xfer.ID, filepath.Base(mergableFile.filepath), traceNumber); err != nil {
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
func (c *fileTransferController) mergeMicroDeposit(mergedDir string, mc uploadableMicroDeposit, depRepo *SQLDepositoryRepo) *achFile {
	file, err := c.loadIncomingFile(mc.fileID)
	if err != nil {
		c.logger.Log("mergeMicroDeposit", fmt.Sprintf("error reading ACH file=%s: %v", mc.fileID, err))
		return nil
	}
	dep, err := depRepo.getUserDepository(DepositoryID(mc.depositoryID), mc.userID)
	if dep == nil || err != nil {
		c.logger.Log("mergeMicroDeposit", fmt.Sprintf("problem reading micro-deposit depository=%s: %v", mc.depositoryID, err))
		return nil
	}

	// Find (or create) a mergable file for this transfer's destination
	mergableFile, err := grabLatestMergedACHFile(dep.RoutingNumber, file, mergedDir) // TODO(adam): is this dep.RoutingNumber the odfiAccount.RoutingNumber (our ODFI's oritin)
	if err != nil {
		c.logger.Log("mergeMicroDeposit", "unable to find mergable file for micro-deposit", "userId", mc.userID, "error", err)
		return nil
	}
	// Merge our transfer's file into mergableFile
	fileToUpload, err := c.mergeTransfer(file, mergableFile)
	if err != nil {
		c.logger.Log("mergeMicroDeposit", fmt.Sprintf("problem during micro-deposit merging: %v", err))
		return nil
	}
	// Mark the micro-deposit as merged and record in which merged file
	if err := depRepo.markMicroDepositAsMerged(filepath.Base(mergableFile.filepath), mc); err != nil {
		c.logger.Log("mergeMicroDeposit", fmt.Sprintf("BAD ERROR - unable to mark micro-deposit as merged: %v", err), "userId", mc.userID)
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

// startUpload looks for ACH files which are ready to be uploaded and matches a filetransfer.CutoffTime
// to them (so we can find their upload configs).
//
// After uploading a file this method renames it to avoid uploading the file multiple times.
func (c *fileTransferController) startUpload(filesToUpload []*achFile) error {
	for i := range filesToUpload {
		for j := range c.cutoffTimes {
			if filesToUpload[i].Header.ImmediateOrigin == c.cutoffTimes[j].RoutingNumber {
				if err := c.maybeUploadFile(filesToUpload[i], c.cutoffTimes[j]); err != nil {
					return fmt.Errorf("problem uploading %s: %v", filesToUpload[i].filepath, err)
				}
				// rename the file so grabLatestMergedACHFile ignores it next time
				if err := os.Rename(filesToUpload[i].filepath, filesToUpload[i].filepath+".uploaded"); err != nil {
					// This is a bad error to run into as it means the file will likely be uploaded twice, but if
					// the underlying FS is failing what other errors would paygate run into?
					return fmt.Errorf("error renaming %s after upload: %v", filesToUpload[i].filepath, err)
				}
			}
		}
	}
	return nil
}

// maybeUploadFile will grab the needed configs and upload an given file to the ODFI's server
func (c *fileTransferController) maybeUploadFile(fileToUpload *achFile, cutoffTime *filetransfer.CutoffTime) error {
	cfg := c.findFileTransferConfig(cutoffTime)
	if cfg == nil {
		return fmt.Errorf("missing file transfer config for %s", cutoffTime.RoutingNumber)
	}
	agent, err := filetransfer.New(c.logger, c.findTransferType(cutoffTime.RoutingNumber), cfg, c.repo)
	if err != nil {
		return fmt.Errorf("problem creating fileTransferAgent for %s: %v", cfg.RoutingNumber, err)
	}
	defer agent.Close()

	c.logger.Log("maybeUploadFile", fmt.Sprintf("uploading %s for routing number %s", fileToUpload.filepath, cutoffTime.RoutingNumber))

	// TODO(adam): I think we should have a DB table for tracking file uploads (?ach_file_uploads?)
	// with the following fields: routing number, filename, timestamp.

	return c.uploadFile(agent, fileToUpload)
}

func (c *fileTransferController) uploadFile(agent filetransfer.Agent, f *achFile) error {
	fd, err := os.Open(f.filepath)
	if err != nil {
		return fmt.Errorf("problem opening %s for upload: %v", f.filepath, err)
	}
	defer fd.Close()

	if err := agent.UploadFile(filetransfer.File{Filename: filepath.Base(f.filepath), Contents: fd}); err != nil {
		return fmt.Errorf("problem uploading %s: %v", f.filepath, err)
	}
	c.logger.Log("uploadFile", fmt.Sprintf("merged: uploaded file %s", f.filepath))
	filesUploaded.With("origin", f.Header.ImmediateOrigin, "destination", f.Header.ImmediateDestination).Add(1)
	return nil
}

// achFilename returns a filename for a given ACH file
//
// yyyy = Year of file creation
// MM = Month of file creation
// dd = Day of file creation
// RTN . . . = 9-digit Routing Transit Number of the bank (ODFI or RDFI) (example: 301234567)
// X = file sequence of the day, i.e., 1, 2, 3, ..., 9, A, B, ...
//
// Full Example: 20181222-301234567-1.ach
func achFilename(routingNumber string, seq int) string {
	s := fmt.Sprintf("%d", seq) // conver to string
	if seq > 9 {
		s = achFilenameSeqToStr(seq)
	}
	return fmt.Sprintf("%s-%s-%s.ach", time.Now().Format("20060102"), routingNumber, s)
}

// achFilenameSeqToStr converts a sequence (int) to it's string value, which means 0-9 followed by A-Z
func achFilenameSeqToStr(seq int) string {
	if seq < 10 {
		return fmt.Sprintf("%d", seq)
	}
	// 65 is ASCII/UTF-8 value for A
	return string(65 + seq - 10) // A, B, ...
}

// achFilenameSeq returns the sequence number from a given achFilename
// A sequence number of 0 indicates an error
func achFilenameSeq(filename string) int {
	parts := strings.Split(filename, "-")
	if len(parts) < 3 {
		return 0
	}
	if parts[2] >= "A" && parts[2] <= "Z" {
		return int(parts[2][0]) - 65 + 10 // A=65 in ASCII/UTF-8
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
// This function will ignore files that don't end with '*.ach'
func grabLatestMergedACHFile(originRoutingNumber string, incoming *ach.File, dir string) (*achFile, error) {
	matches, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("*-%s-*.ach", originRoutingNumber)))
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
			filepath: filepath.Join(dir, achFilename(originRoutingNumber, 1)),
		}

		// We need to increment the FileIDModifier in the FileHeader when creating a new file.
		mergableFile.Header.FileIDModifier = achFilenameSeqToStr(achFilenameSeq(filepath.Base(mergableFile.filepath))) // 0-9 followed by A-Z

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

// groupTransfers will return groupableTransfers grouped according to their origin RoutingNumber
func groupTransfers(xfers []*groupableTransfer, err error) ([][]*groupableTransfer, error) {
	if err != nil {
		return nil, err
	}
	var out [][]*groupableTransfer
	for i := range xfers {
		inserted := false
		for j := range out {
			if xfers[i].origin == out[j][0].origin {
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

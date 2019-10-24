// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	missingFileUploadConfigs = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "missing_ach_file_upload_configs",
		Help: "Counter of missing configurations for file upload - ftp, sftp, or file transfer config(s)",
	}, []string{"routing_number"})
)

// Controller is a controller which is responsible for periodic sync'ing of ACH files
// with their remote FTP/SFTP destination. The ACH network operates on uploading and downloding files
// from hosts during the business day.
type Controller struct {
	// rootDir is the directory where this controller can create scratch files in
	rootDir string

	// batchSize is the number of transfers or micro-deposits to pull from the
	// database and operate on at a time.
	batchSize int

	// interval is how often to pull records from the database and operate on
	interval time.Duration

	config *config.Config

	repo Repository

	ach            *achclient.ACH
	achConfig      *config.ACHConfig
	accountsClient internal.AccountsClient

	logger log.Logger
}

// achFileLineLimit is the maximum line count before an ACH file is uploaded
// to its remote server. NACHA guidelines have a hard limit of 10,000 lines.
func (c *Controller) achFileLineLimit() int {
	if c == nil || c.achConfig == nil || c.achConfig.MaxLines == 0 {
		return 10000
	}
	return c.achConfig.MaxLines
}

// cutoffForceThreshold is the duration before a cutoff time where an ACH file is uploaded
// without merging into a file.
func (c *Controller) cutoffForceThreshold() time.Duration {
	if c == nil || c.achConfig == nil || c.achConfig.ForcedCutoffUploadDelta == 0 {
		return 5 * time.Minute
	}
	return c.achConfig.ForcedCutoffUploadDelta
}

// NewController returns a Controller which is responsible for uploading ACH files
// to their SFTP host for processing.
//
// To change the refresh duration set ACH_FILE_TRANSFER_INTERVAL with a Go time.Duration value. (i.e. 10m for 10 minutes)
func NewController(logger log.Logger, config *config.Config, dir string, repo Repository, achClient *achclient.ACH, accountsClient internal.AccountsClient, accountsCallsDisabled bool) (*Controller, error) {
	if _, err := os.Stat(dir); dir == "" || err != nil {
		return nil, fmt.Errorf("file-transfer-controller: problem with storage directory %q: %v", dir, err)
	}

	var interval time.Duration
	if v := os.Getenv("ACH_FILE_TRANSFER_INTERVAL"); strings.EqualFold(v, "off") || strings.EqualFold(v, "0m") {
		logger.Log("file-transfer-controller", "disabling Controller via config (ACH_FILE_TRANSFER_INTERVAL)")
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
	logger.Log("NewController", fmt.Sprintf("starting ACH file transfer controller: interval=%v batchSize=%d", interval, batchSize))

	rootDir, err := filepath.Abs(dir)
	if err != nil || strings.Contains(dir, "..") {
		return nil, fmt.Errorf("file-transfer-controller: invalid directory %s: %v", dir, err)
	}
	if err := os.MkdirAll(rootDir, 0777); err != nil {
		return nil, fmt.Errorf("file-transfer-controller: error creating %s: %v", rootDir, err)
	}

	controller := &Controller{
		config:    config,
		rootDir:   rootDir,
		interval:  interval,
		batchSize: batchSize,
		repo:      repo,
		ach:       achClient,
		logger:    logger,
	}
	if !accountsCallsDisabled {
		controller.accountsClient = accountsClient
	}
	return controller, nil
}

func (c *Controller) findFileTransferConfig(routingNumber string) *Config {
	cfgs, err := c.repo.GetConfigs()
	if err != nil {
		return nil
	}
	for i := range cfgs {
		if cfgs[i].RoutingNumber == routingNumber {
			return cfgs[i]
		}
	}
	return nil
}

// findTransferType will return a string from matching the provided routingNumber against
// FTP, SFTP (and future) file transport protocols. This string needs to match New.
func (c *Controller) findTransferType(routingNumber string) string {
	ftpConfigs, err := c.repo.GetFTPConfigs()
	if err != nil {
		return fmt.Sprintf("unknown: error=%v", err)
	}
	for i := range ftpConfigs {
		if ftpConfigs[i].RoutingNumber == routingNumber {
			return "ftp"
		}
	}

	sftpConfigs, err := c.repo.GetSFTPConfigs()
	if err != nil {
		return fmt.Sprintf("unknown: error=%v", err)
	}
	for i := range sftpConfigs {
		if sftpConfigs[i].RoutingNumber == routingNumber {
			return "sftp"
		}
	}

	return "unknown"
}

type FlushChan chan *periodicFileOperationsRequest

type periodicFileOperationsRequest struct {
	// requestID is the x-request-id HTTP header
	requestID string

	// userID is the x-user-id HTTP header
	userID string

	// waiter is an optional channel to signal when the file operations
	// are completed. This is used to hold HTTP responses (for the admin
	// endpoints).
	waiter chan struct{}
}

// StartPeriodicFileOperations will block forever to periodically download incoming and returned ACH files while also merging
// and uploading ACH files to their remote SFTP server. forceUpload is a channel for manually triggering the "merge and upload"
// portion of this pooling loop, which is used by admin endpoints and to make testing easier.
//
// Uploads will be completed before their cutoff time which is set for a given ABA routing number.
func (c *Controller) StartPeriodicFileOperations(ctx context.Context, flushIncoming FlushChan, flushOutgoing FlushChan, depRepo internal.DepositoryRepository, transferRepo internal.TransferRepository) {
	tick := time.NewTicker(c.interval)
	defer tick.Stop()

	// Grab shared transfer cursor for new transfers to merge into local files
	transferCursor := transferRepo.GetTransferCursor(c.batchSize, depRepo)
	microDepositCursor := depRepo.GetMicroDepositCursor(c.batchSize)

	finish := func(req *periodicFileOperationsRequest, wg *sync.WaitGroup, errs chan error) {
		// Wait for all operations to complete
		wg.Wait()

		requestID, userID := "", ""
		if req != nil {
			requestID = req.requestID
			userID = req.userID
		}

		errs <- nil // send so channel read doesn't block
		if err := <-errs; err != nil {
			c.logger.Log("StartPeriodicFileOperations", fmt.Sprintf("ERROR: periodic file operation"), "requestID", requestID, "userID", userID, "error", err)
		} else {
			c.logger.Log("StartPeriodicFileOperations", fmt.Sprintf("files sync'd, waiting %v", c.interval), "requestID", requestID, "userID", userID)
		}
		if req != nil && req.waiter != nil {
			req.waiter <- struct{}{} // signal to our waiter the request is finished
		}
	}

	for {
		// Setup our concurrnet waiting
		var wg sync.WaitGroup
		errs := make(chan error, 10)

		select {
		case req := <-flushIncoming:
			c.logger.Log("StartPeriodicFileOperations", "flushing inbound ACH files", "requestID", req.requestID, "userID", req.userID)
			if err := c.downloadAndProcessIncomingFiles(req, depRepo, transferRepo); err != nil {
				errs <- fmt.Errorf("downloadAndProcessIncomingFiles: %v", err)
			}
			finish(req, &wg, errs)

		case req := <-flushOutgoing:
			c.logger.Log("StartPeriodicFileOperations", "flushing ACH files to their outbound destination", "requestID", req.requestID, "userID", req.userID)
			if err := c.mergeAndUploadFiles(transferCursor, microDepositCursor, transferRepo, req, &mergeUploadOpts{force: true}); err != nil {
				errs <- fmt.Errorf("mergeAndUploadFiles: %v", err)
			}
			finish(req, &wg, errs)

		case <-tick.C:
			// This is triggered by the time.Ticker (which accounts for delays) so let's download and upload files.
			c.logger.Log("StartPeriodicFileOperations", "Starting periodic file operations")
			req := &periodicFileOperationsRequest{}
			wg.Add(1)
			go func() {
				if err := c.downloadAndProcessIncomingFiles(req, depRepo, transferRepo); err != nil {
					errs <- fmt.Errorf("downloadAndProcessIncomingFiles: %v", err)
				}
				wg.Done()
			}()
			// Grab transfers, merge them into files, and upload any which are complete.
			wg.Add(1)
			go func() {
				if err := c.mergeAndUploadFiles(transferCursor, microDepositCursor, transferRepo, req, &mergeUploadOpts{}); err != nil {
					errs <- fmt.Errorf("mergeAndUploadFiles: %v", err)
				}
				wg.Done()
			}()
			finish(nil, &wg, errs)

		case <-ctx.Done():
			c.logger.Log("StartPeriodicFileOperations", "Shutting down due to context.Done()")
			return
		}
	}
}

// writeFiles will create files in dir for each file object provided
// The contents of each file struct will always be closed.
func (c *Controller) writeFiles(files []File, dir string) error {
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
		if err := f.Sync(); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		if err := files[i].Contents.Close(); err != nil {
			return err
		}
	}
	if len(errordFilenames) != 0 {
		return fmt.Errorf("writeFiles problem on: %s: %v", strings.Join(errordFilenames, ", "), firstErr)
	}
	return nil
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

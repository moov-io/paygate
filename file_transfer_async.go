// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
)

// cutoffTime represents the time of a banking day when all ACH files need to be uploaded in order
// to be processed for that day. Files which miss the cutoff time won't be processed until the next day.
type cutoffTime struct {
	routingNumber string
	cutoff        int            // 24-hour time value (0000 to 2400)
	loc           *time.Location // timezone cutoff is in (usually America/New_York)
}

// fileTransferController is a controller which is responsible for periodic sync'ing of ACH files
// with their remote SFTP destination. The ACH network operates on uploading and downloding files
// from hosts during the business day.
type fileTransferController struct {
	rootDir string

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
	logger.Log("file-transfer-controller", fmt.Sprintf("starting ACH file transfer controller with interval %v", interval))

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

	return &fileTransferController{
		interval:            interval,
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

// startPeriodicFileSync will block forever and periodically sync the ACH files with their remote SFTP server.
// This will be done before the cutoff time which is set for a given ABA routing number
func (c *fileTransferController) startPeriodicFileSync(transferRepo transferRepository) {
	for {
		time.Sleep(c.interval)
		if err := c.syncFiles(transferRepo); err != nil {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("ERROR: syncing files: %v", err))
		} else {
			c.logger.Log("file-transfer-controller", fmt.Sprintf("files sync'd, waiting %v", c.interval))
		}
	}
}

// syncFiles will upload and download all ACH files to their FTP servers.
// This method initiates the download and upload steps and blocks until either
// both complete or error.
func (c *fileTransferController) syncFiles(transferRepo transferRepository) error {
	var wg sync.WaitGroup

	// For all routing numbers grab their inbound and return files
	if err := c.downloadAllFiles(&wg); err != nil {
		return err
	}

	// Setup file uploader

	// transferCursor, err := transferRepo.getTransferCursor()
	//
	// func (agent *fileTransferAgent) uploadFile(f file) error
	//
	// func (a *ACH) GetFileContents(fileId string) (*bytes.Buffer, error)

	// steps
	// 1. get transfers that need to be posted today
	// 1. <group by ABA>
	//   1. grab those ACH files from service
	//   1. upload to SFTP server

	// for each ABA get inbound and return files for parsing
	// can update transfer status, send alerts?

	wg.Wait()
	return nil
}

// downloadAllFiles will setup directories for each routing number and initiate downloading and
// writing the files to sub-directories.
func (c *fileTransferController) downloadAllFiles(wg *sync.WaitGroup) error {
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
			return fmt.Errorf("file-transfer-controller: problem with %s file transfer agent init: %v", c.cutoffTimes[i].routingNumber, err)
		}
		defer agent.close()

		dir := filepath.Join(c.rootDir, c.cutoffTimes[i].routingNumber)
		os.Mkdir(dir, 0777) // ignore failure if dir already exists // TODO(adam): should we always delete it first?

		// Setup file downloads
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.saveRemoteFiles(agent, dir); err != nil {
				c.logger.Log("file-transfer-controller", fmt.Sprintf("ERROR downloading files for %s: %v", c.cutoffTimes[i].routingNumber, err))
			}
		}()
	}
	return nil
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
	errs := make(chan error)
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

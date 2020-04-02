// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/moov-io/paygate/internal/filetransfer/config"
	"github.com/moov-io/paygate/internal/filetransfer/upload"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	inboundFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "inbound_ach_files_processed",
		Help: "Counter of inbound files processed by paygate",
	}, []string{"destination", "origin", "code"})

	missingFileUploadConfigs = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "missing_ach_file_upload_configs",
		Help: "Counter of missing configurations for file upload - ftp, sftp, or file transfer config(s)",
	}, []string{"routing_number"})
)

// downloadAndProcessIncomingFiles will take each cutoffTime initialized with the controller and retrieve all files
// on the remote server for them. After this method will call processInboundFiles and processReturnFiles on each
// downloaded file.
func (c *Controller) downloadAndProcessIncomingFiles(req *periodicFileOperationsRequest) error {
	dir, err := ioutil.TempDir(c.rootDir, "downloaded")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	cutoffTimes, err := c.repo.GetCutoffTimes()
	if err != nil {
		return fmt.Errorf("cutoff times: %v", err)
	}
	for i := range cutoffTimes {
		fileTransferConf := c.findFileTransferConfig(cutoffTimes[i].RoutingNumber)
		if fileTransferConf == nil {
			c.logger.Log(
				"downloadAndProcessIncomingFiles", fmt.Sprintf("missing file transfer config for %s", cutoffTimes[i].RoutingNumber),
				"userID", req.userID, "requestID", req.requestID)
			missingFileUploadConfigs.With("routing_number", cutoffTimes[i].RoutingNumber).Add(1)
			continue
		}
		if err := c.downloadAllFiles(dir, fileTransferConf); err != nil {
			c.logger.Log(
				"downloadAndProcessIncomingFiles", fmt.Sprintf("error downloading files into %s", dir), "error", err,
				"userID", req.userID, "requestID", req.requestID)
			continue
		}

		// Read and process inbound and returned files
		if err := c.processInboundFiles(req, filepath.Join(dir, fileTransferConf.InboundPath)); err != nil {
			c.logger.Log(
				"downloadAndProcessIncomingFiles", fmt.Sprintf("problem reading inbound files in %s", dir), "error", err,
				"userID", req.userID, "requestID", req.requestID)
			continue
		}
		if err := c.processReturnFiles(filepath.Join(dir, fileTransferConf.ReturnPath)); err != nil {
			c.logger.Log(
				"downloadAndProcessIncomingFiles", fmt.Sprintf("problem reading return files in %s", dir), "error", err,
				"userID", req.userID, "requestID", req.requestID)
			continue
		}
	}

	return nil
}

// downloadAllFiles will setup directories for each routing number and initiate downloading and writing the files to sub-directories.
func (c *Controller) downloadAllFiles(dir string, fileTransferConf *config.Config) error {
	agentType := c.findAgentType(fileTransferConf.RoutingNumber)
	agent, err := upload.New(c.logger, agentType, fileTransferConf, c.repo)
	if err != nil {
		return fmt.Errorf("downloadAllFiles: problem with %s %s file transfer agent init: %v", fileTransferConf.RoutingNumber, agentType, err)
	}
	defer agent.Close()

	// Setup file downloads
	if err := c.saveRemoteFiles(agent, dir); err != nil {
		c.logger.Log("downloadAllFiles", fmt.Sprintf("ERROR downloading files over %s (ABA: %s)", agentType, fileTransferConf.RoutingNumber), "error", err)
	}
	return nil
}

func (c *Controller) processInboundFiles(req *periodicFileOperationsRequest, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if (err != nil && err != filepath.SkipDir) || info.IsDir() {
			return nil // Ignore SkipDir and directories
		}

		file, err := parseACHFilepath(path)
		if err != nil {
			c.logger.Log(
				"processInboundFiles", fmt.Sprintf("problem parsing inbound file %s", path), "error", err,
				"userID", req.userID, "requestID", req.requestID)
			return nil
		}
		c.logger.Log(
			"file-transfer-controller", fmt.Sprintf("processing inbound file %s from %s (%s)", info.Name(), file.Header.ImmediateOriginName, file.Header.ImmediateOrigin),
			"userID", req.userID, "requestID", req.requestID)

		// Handle any NOC Batches
		if len(file.NotificationOfChange) > 0 {
			inboundFilesProcessed.With("destination", file.Header.ImmediateDestination, "origin", file.Header.ImmediateOrigin, "code", "").Add(1) // TODO(adam):
			if err := c.handleNOCFile(req, file, info.Name()); err != nil {
				c.logger.Log(
					"processInboundFiles", fmt.Sprintf("problem with inbound NOC file %s", path), "error", err,
					"userID", req.userID, "requestID", req.requestID)
			}
		} else {
			c.logger.Log(
				"file-transfer-controller", fmt.Sprintf("skipping file %s with zero NOC entres", info.Name()),
				"userID", req.userID, "requestID", req.requestID)
		}

		return nil
	})
}

// saveRemoteFiles will write all inbound and return ACH files for a given routing number to the specified directory
func (c *Controller) saveRemoteFiles(agent upload.Agent, dir string) error {
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

// writeFiles will create files in dir for each file object provided
// The contents of each file struct will always be closed.
func (c *Controller) writeFiles(files []upload.File, dir string) error {
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

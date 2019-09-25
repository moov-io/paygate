// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	inboundFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "inbound_ach_files_processed",
		Help: "Counter of inbound files processed by paygate",
	}, []string{"destination", "origin"})
)

// downloadAndProcessIncomingFiles will take each cutoffTime initialized with the controller and retrieve all files
// on the remote server for them. After this method will call processInboundFiles and processReturnFiles on each
// downloaded file.
func (c *Controller) downloadAndProcessIncomingFiles(depRepo internal.DepositoryRepository, transferRepo internal.TransferRepository) error {
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
func (c *Controller) downloadAllFiles(dir string, fileTransferConf *Config) error {
	agent, err := New(c.logger, c.findTransferType(fileTransferConf.RoutingNumber), fileTransferConf, c.repo)
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

func (c *Controller) processInboundFiles(dir string) error {
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

// saveRemoteFiles will write all inbound and return ACH files for a given routing number to the specified directory
func (c *Controller) saveRemoteFiles(agent Agent, dir string) error {
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
func (c *Controller) loadIncomingFile(fileId string) (*ach.File, error) {
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

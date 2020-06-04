// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/upload"

	"github.com/go-kit/kit/log"
)

// import (
// 	"github.com/go-kit/kit/metrics/prometheus"
// 	stdprometheus "github.com/prometheus/client_golang/prometheus"
// )

// var (
// 	inboundFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
// 		Name: "inbound_ach_files_processed",
// 		Help: "Counter of inbound files processed",
// 	}, []string{"origin", "destination"})
// )

// inboundFilesProcessed.With("origin", file.Header.ImmediateOrigin, "destination", file.Header.ImmediateDestination).Add(1)

// setup to read files from remote service and send off as COR/NOC, prenote, or transfer

type Downloader interface {
	CopyFilesFromRemote(agent upload.Agent) (*downloadedFiles, error)
}

func NewDownloader(logger log.Logger, cfg *config.Storage) Downloader {
	var baseDir string
	if cfg != nil && cfg.Local != nil {
		baseDir = cfg.Local.Directory
	}
	return &downloaderImpl{
		logger:  logger,
		baseDir: baseDir,
	}
}

type downloaderImpl struct {
	logger  log.Logger
	baseDir string
}

// downloadedFiles is a randomly generated directory inside of the storage directory.
// These are designed to be deleted after all files are processed.
type downloadedFiles struct {
	dir string
}

func (d *downloadedFiles) Close() error {
	return os.RemoveAll(d.dir)
}

func (dl *downloaderImpl) setup(agent upload.Agent) (*downloadedFiles, error) {
	dir, err := ioutil.TempDir(dl.baseDir, "download")
	if err != nil {
		return nil, err
	}

	// Create sub-directories for files we download
	path := filepath.Join(dir, agent.InboundPath())
	if err := os.Mkdir(path, 0777); err != nil {
		return nil, fmt.Errorf("problem creating %s: %v", path, err)
	}
	path = filepath.Join(dir, agent.ReturnPath())
	if err := os.Mkdir(path, 0777); err != nil {
		return nil, fmt.Errorf("problem creating %s: %v", path, err)
	}

	return &downloadedFiles{
		dir: dir,
	}, nil
}

func (dl *downloaderImpl) CopyFilesFromRemote(agent upload.Agent) (*downloadedFiles, error) {
	out, err := dl.setup(agent)
	if err != nil {
		return nil, err
	}

	// copy down files from our "inbound" directory
	files, err := agent.GetInboundFiles()
	dl.logger.Log("download", fmt.Sprintf("found %d inbound files", len(files)))
	if err != nil {
		return out, fmt.Errorf("problem downloading inbound files: %v", err)
	}
	if err := dl.writeFiles(filepath.Join(out.dir, agent.InboundPath()), files); err != nil {
		return out, fmt.Errorf("problem saving inbound files: %v", err)
	}

	// copy down files from out "return" directory
	files, err = agent.GetReturnFiles()
	dl.logger.Log("download", fmt.Sprintf("found %d return files", len(files)))
	if err != nil {
		return out, fmt.Errorf("problem downloading return files: %v", err)
	}
	if err := dl.writeFiles(filepath.Join(out.dir, agent.ReturnPath()), files); err != nil {
		return out, fmt.Errorf("problem saving return files: %v", err)
	}

	return out, nil
}

// writeFiles will create files in dir for each file object provided
// The contents of each file struct will always be closed.
func (dl *downloaderImpl) writeFiles(dir string, files []upload.File) error {
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
		dl.logger.Log("inbound", fmt.Sprintf("saved %s at %s", files[i].Filename, filepath.Join(dir, files[i].Filename)))
	}
	if len(errordFilenames) != 0 {
		return fmt.Errorf("writeFiles problem on: %s: %v", strings.Join(errordFilenames, ", "), firstErr)
	}
	return nil
}

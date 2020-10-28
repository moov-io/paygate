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

	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/moov-io/base/log"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	filesDownloaded = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "files_downloaded",
		Help: "Counter of files downloaded from a remote server",
	}, []string{"kind"})
)

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

func (d *downloadedFiles) deleteFiles() error {
	return os.RemoveAll(d.dir)
}

func (d *downloadedFiles) deleteEmptyDirs(agent upload.Agent) error {
	count := func(path string) int {
		infos, err := ioutil.ReadDir(path)
		if err != nil {
			return -1
		}
		return len(infos)
	}
	if path := filepath.Join(d.dir, agent.InboundPath()); count(path) == 0 {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("delete inbound: %v", err)
		}
	}
	if path := filepath.Join(d.dir, agent.ReturnPath()); count(path) == 0 {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("delete return: %v", err)
		}
	}
	return nil
}

func (dl *downloaderImpl) setup(agent upload.Agent) (*downloadedFiles, error) {
	dir, err := ioutil.TempDir(dl.baseDir, "download")
	if err != nil {
		return nil, err
	}

	// Create sub-directories for files we download
	path := filepath.Join(dir, agent.InboundPath())
	if err := os.MkdirAll(path, 0777); err != nil {
		return nil, fmt.Errorf("problem creating %s: %v", path, err)
	}
	path = filepath.Join(dir, agent.ReturnPath())
	if err := os.MkdirAll(path, 0777); err != nil {
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
	dl.logger.Logf("found %d inbound files", len(files))
	if err != nil {
		return out, fmt.Errorf("problem downloading inbound files: %v", err)
	}
	filesDownloaded.With("kind", "inbound").Add(float64(len(files)))
	if err := dl.writeFiles(filepath.Join(out.dir, agent.InboundPath()), files); err != nil {
		return out, fmt.Errorf("problem saving inbound files: %v", err)
	}

	// copy down files from out "return" directory
	files, err = agent.GetReturnFiles()
	dl.logger.Logf("found %d return files", len(files))
	if err != nil {
		return out, fmt.Errorf("problem downloading return files: %v", err)
	}
	filesDownloaded.With("kind", "return").Add(float64(len(files)))
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
		dl.logger.Logf("saved %s at %s", files[i].Filename, filepath.Join(dir, files[i].Filename))
	}
	if len(errordFilenames) != 0 {
		return fmt.Errorf("writeFiles problem on: %s: %v", strings.Join(errordFilenames, ", "), firstErr)
	}
	return nil
}

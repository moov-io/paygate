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

type Download struct {
	localDirectory string
}

func (dl *Download) setup(root string, agent upload.Agent) error {
	dl.localDirectory = root

	path := filepath.Join(dl.localDirectory, agent.InboundPath())
	if err := os.Mkdir(path, 0777); err != nil {
		return fmt.Errorf("problem creating %s: %v", path, err)
	}

	path = filepath.Join(dl.localDirectory, agent.ReturnPath())
	if err := os.Mkdir(path, 0777); err != nil {
		return fmt.Errorf("problem creating %s: %v", path, err)
	}

	return nil
}

func (dl *Download) Close() error {
	if dl == nil || dl.localDirectory == "" {
		return nil
	}
	return os.RemoveAll(dl.localDirectory)
}

func (dl *Download) String() string {
	return fmt.Sprintf(`Download{localDirectory=%s}`, dl.localDirectory)
}

type Downloader interface {
	CopyFilesFromRemote(agent upload.Agent) (*Download, error)
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

func (d *downloaderImpl) startDownload(agent upload.Agent) *Download {
	os.MkdirAll(d.baseDir, 0777)
	dir, _ := ioutil.TempDir(d.baseDir, "download")
	dl := &Download{
		localDirectory: dir,
	}
	dl.setup(dir, agent)
	return dl
}

func (dl *downloaderImpl) CopyFilesFromRemote(agent upload.Agent) (*Download, error) {
	out := dl.startDownload(agent)

	// copy down files from our "inbound" directory
	files, err := agent.GetInboundFiles()
	dl.logger.Log("download", fmt.Sprintf("found %d inbound files", len(files)))
	if err != nil {
		return out, fmt.Errorf("problem downloading inbound files: %v", err)
	}
	if err := dl.writeFiles(filepath.Join(out.localDirectory, agent.InboundPath()), files); err != nil {
		return out, fmt.Errorf("problem saving inbound files: %v", err)
	}

	// copy down files from out "return" directory
	files, err = agent.GetReturnFiles()
	dl.logger.Log("download", fmt.Sprintf("found %d return files", len(files)))
	if err != nil {
		return out, fmt.Errorf("problem downloading return files: %v", err)
	}
	if err := dl.writeFiles(filepath.Join(out.localDirectory, agent.ReturnPath()), files); err != nil {
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

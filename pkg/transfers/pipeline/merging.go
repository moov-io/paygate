// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"

	"github.com/go-kit/kit/log"
)

// XferMerging represents logic for accepting ACH files to be merged together.
//
// The idea is to take Xfers and store them on a filesystem (or other durable storage)
// prior to a cutoff window. The specific storage could be based on the FileHeader.
//
// On the cutoff trigger WithEachMerged is called to merge files together and offer
// each merged file for an upload.
type XferMerging interface {
	HandleXfer(xfer Xfer) error
	HandleCancel(xfer Xfer) error

	// Reset() error
	WithEachMerged(func(*ach.File) error) error
}

func NewMerging(logger log.Logger, cfg config.Pipeline) (XferMerging, error) {
	dir := filepath.Join("storage", "mergable") // default directory
	if cfg.Filesystem != nil {
		dir = filepath.Join(cfg.Filesystem.Directory, "mergable")
	}

	if err := os.MkdirAll(dir, 0777); err != nil {
		return nil, err
	}

	return &filesystemMerging{
		baseDir: dir,
		logger:  logger,
	}, nil
}

type filesystemMerging struct {
	logger  log.Logger
	baseDir string
}

// cfg *config.FilesystemPipeline
//   filepath.Join(cfg.Directory, "mergable")
//
// create 'FileHeader.Hash() string' ?

// TODO(adam): this has one impl, take Xfer (or it's canceled form) and write to fs
// so we can merge outbound files the same.

func (m *filesystemMerging) HandleXfer(xfer Xfer) error {
	err1 := m.writeTransfer(xfer.Transfer)
	err2 := m.writeACHFile(xfer.Transfer.TransferID, xfer.File)

	if err1 != nil || err2 != nil {
		return fmt.Errorf("problem writing transfer: %v\n problem writing ACH file: %v", err1, err2)
	}

	return nil
}

func (m *filesystemMerging) writeTransfer(transfer *client.Transfer) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(transfer); err != nil {
		return err
	}

	path := filepath.Join(m.baseDir, fmt.Sprintf("%s.json", transfer.TransferID))
	if err := ioutil.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

func (m *filesystemMerging) writeACHFile(transferID string, file *ach.File) error {
	var buf bytes.Buffer
	if err := ach.NewWriter(&buf).Write(file); err != nil {
		return err
	}

	path := filepath.Join(m.baseDir, fmt.Sprintf("%s.ach", transferID))
	if err := ioutil.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

func (m *filesystemMerging) HandleCancel(xfer Xfer) error {
	return nil
}

func (m *filesystemMerging) isolateMergableDir() (string, error) {
	// rename m.baseDir so we're the only accessor for it, then recreate m.baseDir
	parent, _ := filepath.Split(m.baseDir)
	newdir := filepath.Join(parent, time.Now().Format("20060102-150405"))
	if err := os.Rename(m.baseDir, newdir); err != nil {
		return newdir, err
	}
	return newdir, os.Mkdir(m.baseDir, 0777) // create m.baseDir again
}

func (m *filesystemMerging) WithEachMerged(f func(*ach.File) error) error {
	// move the current directory so it's isolated and easier to debug later on
	dir, err := m.isolateMergableDir()
	if err != nil {
		return fmt.Errorf("problem isolating newdir=%s error=%v", dir, err)
	}
	path := filepath.Join(dir, "*.ach") // TODO(adam): exclude matches with '*.canceled' files

	matches, err := filepath.Glob(path)
	if err != nil {
		return fmt.Errorf("problem with %s glob: %v", path, err)
	}

	var files []*ach.File
	var el base.ErrorList
	for i := range matches {
		file, err := ach.ReadFile(matches[i])
		if err != nil {
			el.Add(fmt.Errorf("problem reading %s: %v", matches[i], err))
			continue
		}
		if file != nil {
			files = append(files, file)
		}
	}
	files, err = ach.MergeFiles(files)
	if err != nil {
		el.Add(fmt.Errorf("unable to merge files: %v", err))
	}

	if len(matches) > 0 {
		m.logger.Log("merging", fmt.Sprintf("merged %d transfers into %d files", len(matches), len(files)))
	}
	if len(files) == 0 {
		// delete the new directory as there's nothing to merge
		if err := os.RemoveAll(dir); err != nil {
			el.Add(err)
		}
	}

	for i := range files {
		// TODO(adam): write each merged file here?

		if err := f(files[i]); err != nil {
			el.Add(fmt.Errorf("problem from callback: %v", err))
		}
	}

	if !el.Empty() {
		return el
	}

	return nil
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"github.com/moov-io/ach"
)

type XferMerging interface {
	HandleXfer(xfer Xfer) error
	HandleCancel(xfer Xfer) error

	WithEachMerged(func(*ach.File) error) error
}

type filesystemMerging struct {
	baseDir string
}

// cfg *config.FilesystemPipeline
//   filepath.Join(cfg.Directory, "mergable")
//
// create 'FileHeader.Hash() string' ?

// TODO(adam): this has one impl, take Xfer (or it's canceled form) and write to fs
// so we can merge outbound files the same.

func (m *filesystemMerging) HandleXfer(xfer Xfer) error {
	return nil
}

func (m *filesystemMerging) HandleCancel(xfer Xfer) error {
	return nil
}

func (m *filesystemMerging) WithEachMerged(func(*ach.File) error) error {
	return nil
}

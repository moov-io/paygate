// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moov-io/paygate/pkg/config"
)

type filesystemPublisher struct {
	baseDir string
}

func createFilesystemPublisher(cfg *config.FilesystemPipeline) (*filesystemPublisher, error) {
	pub := &filesystemPublisher{
		baseDir: filepath.Join(cfg.Directory, "xfers"),
	}
	if err := os.MkdirAll(pub.baseDir, 0777); err != nil {
		return pub, fmt.Errorf("problem creating %s: %v", pub.baseDir, err)
	}
	return pub, nil
}

func (fs *filesystemPublisher) Upload(xfer Xfer) error {
	// TODO(adam): write Xfer (as JSON) to directory

	return nil
}

func (fs *filesystemPublisher) Cancel(xfer Xfer) error {
	return nil
}

func (fs *filesystemPublisher) Shutdown(_ context.Context) {}

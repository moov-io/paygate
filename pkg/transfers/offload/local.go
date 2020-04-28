// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package offload

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moov-io/paygate/pkg/config"
)

type localOffloader struct {
	dir      string
	interval time.Duration
}

func createLocalOffloader(cfg *config.Config) (*localOffloader, error) {
	off := &localOffloader{
		interval: cfg.Offloader.Local.Interval,
		dir:      cfg.Offloader.Local.Directory,
	}
	if err := off.setup(); err != nil {
		return nil, fmt.Errorf("local offloader: %v", err)
	}
	return off, nil
}

func (off *localOffloader) setup() error {
	if err := os.MkdirAll(off.outboundDir(), 0777); err != nil {
		return fmt.Errorf("error creating %s: %v", off.outboundDir(), err)
	}
	if err := os.MkdirAll(off.mergedDir(), 0777); err != nil {
		return fmt.Errorf("error creating %s: %v", off.mergedDir(), err)
	}
	return nil
}

func (off *localOffloader) outboundDir() string {
	return filepath.Join(off.dir, "outbound")
}

func (off *localOffloader) mergedDir() string {
	return filepath.Join(off.dir, "merged")
}

func (off *localOffloader) Upload(xfer Xfer) error {
	return nil
}

func (off *localOffloader) Cancel(xfer Xfer) error {
	return nil
}

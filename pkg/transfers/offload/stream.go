// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package offload

import (
	"fmt"

	"github.com/moov-io/paygate/pkg/config"
)

type streamOffloader struct {
	// kafka / gocloud.dev/pubsub stuff
}

func createStreamOffloader(cfg *config.Config) (*streamOffloader, error) {
	off := &streamOffloader{}
	if err := off.setup(); err != nil {
		return nil, fmt.Errorf("stream offloader: %v", err)
	}
	return off, nil
}

func (off *streamOffloader) setup() error {
	// setup stream client

	return nil
}

func (off *streamOffloader) Upload(xfer Xfer) error {
	return nil
}

func (off *streamOffloader) Cancel(xfer Xfer) error {
	return nil
}

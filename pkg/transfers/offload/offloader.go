// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package offload

import (
	"errors"
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

type Offloader interface {
	Upload(xfer Xfer) error
	Cancel(xfer Xfer) error
}

type Xfer struct {
	Transfer *client.Transfer
	File     *ach.File
}

func New(cfg *config.Config) (Offloader, error) {
	switch {
	case cfg == nil:
		return nil, errors.New("nil Config")

	case cfg.Offloader.Local != nil:
		return createLocalOffloader(cfg)

	case cfg.Offloader.Stream != nil:
		return createStreamOffloader(cfg)
	}
	return nil, fmt.Errorf("unknown offloader: %#v", cfg.Offloader)
}

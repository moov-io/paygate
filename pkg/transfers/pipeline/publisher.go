// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"errors"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

type Xfer struct {
	Transfer *client.Transfer `json:"transfer"`
	File     *ach.File        `json:"file"`
}

// XferPublisher ... // TODO(adam):
type XferPublisher interface {
	Upload(xfer Xfer) error
	Cancel(xfer Xfer) error
}

func NewPublisher(cfg *config.Config) (XferPublisher, error) {
	if cfg == nil {
		return nil, errors.New("nil Config")
	}
	if cfg.Pipeline.Filesystem != nil {
		return createFilesystemPublisher(cfg.Pipeline.Filesystem)
	}
	if cfg.Pipeline.Stream != nil {
		return createStreamPublisher(cfg.Pipeline.Stream)
	}
	return nil, errors.New("unknown Pipeline config")
}

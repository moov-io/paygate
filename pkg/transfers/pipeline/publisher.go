// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"context"
	"errors"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

// XferPublisher is an interface for pushing Transfers (and their ACH files) to be
// uploaded to an ODFI. These implementations can be to push Transfers onto streams
// (e.g. kafka, rabbitmq) or inmem (the default in our OSS PayGate).
type XferPublisher interface {
	Upload(xfer Xfer) error
	Cancel(msg CanceledTransfer) error
	Shutdown(ctx context.Context)
}

func NewPublisher(cfg config.Pipeline) (XferPublisher, error) {
	if cfg.Stream != nil {
		return createStreamPublisher(cfg.Stream)
	}
	return nil, errors.New("unknown Pipeline config")
}

// PublishFiles attempts to upload all files to the Pipeline and returns
// all errors as a base.ErrorList.
//
// All files are attempted to be published as downstream processors
// are expected to de-duplicate files.
func PublishFiles(pub XferPublisher, xfer *client.Transfer, files []*ach.File) error {
	if pub == nil {
		return nil
	}

	var el base.ErrorList
	for i := range files {
		xf := Xfer{
			File:     files[i],
			Transfer: xfer,
		}
		if err := pub.Upload(xf); err != nil {
			el.Add(err)
		}
	}
	if el.Empty() {
		return nil
	}
	return el
}

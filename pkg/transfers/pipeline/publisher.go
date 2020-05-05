// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moov-io/paygate/pkg/config"
)

// XferPublisher ... // TODO(adam):
type XferPublisher interface {
	Upload(xfer Xfer) error
	Cancel(xfer Xfer) error // TODO(adam): this needs a different type
	Shutdown(ctx context.Context)
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

func createMetadata(xf Xfer) map[string]string {
	out := make(map[string]string)
	out["transferID"] = xf.Transfer.TransferID
	return out
}

func createBody(xf Xfer) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(xf); err != nil {
		return nil, fmt.Errorf("trasferID=%s json encode: %v", xf.Transfer.TransferID, err)
	}
	return buf.Bytes(), nil
}

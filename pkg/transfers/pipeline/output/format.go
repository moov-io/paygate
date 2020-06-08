// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package output

import (
	"bytes"
	"errors"
	"strings"

	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/transform"
)

// Formatter is a structure for encoding an encrypted or plaintext ACH file.
type Formatter interface {
	Format(buf *bytes.Buffer, res *transform.Result) error
}

func NewFormatter(cfg *config.Output) (Formatter, error) {
	if cfg == nil || cfg.Format == "" {
		return &NACHA{}, nil
	}
	switch {
	case strings.EqualFold(cfg.Format, "base64"):
		return &Base64{}, nil

	case strings.EqualFold(cfg.Format, "encrypted-bytes"):
		return &Encrypted{}, nil

	case strings.EqualFold(cfg.Format, "nacha"):
		return &NACHA{}, nil
	}
	return nil, errors.New("unknown output format")
}

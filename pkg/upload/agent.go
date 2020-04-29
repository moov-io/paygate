// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"fmt"
	"strings"

	"github.com/moov-io/paygate/pkg/config"

	"github.com/go-kit/kit/log"
)

// Agent represents an interface for uploading and retrieving ACH files from a remote service.
type Agent interface {
	GetInboundFiles() ([]File, error)
	GetReturnFiles() ([]File, error)
	UploadFile(f File) error
	Delete(path string) error

	InboundPath() string
	OutboundPath() string
	ReturnPath() string

	Close() error
}

func New(logger log.Logger, _type string, cfg *config.ODFI) (Agent, error) {
	switch strings.ToLower(_type) {
	case "ftp":
		return newFTPTransferAgent(logger, cfg)

	case "sftp":
		return newSFTPTransferAgent(logger, cfg)

	default:
		return nil, fmt.Errorf("filetransfer: unknown type '%s'", _type)
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"errors"

	"github.com/moov-io/paygate/pkg/config"

	"github.com/moov-io/base/log"
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
	Hostname() string

	Ping() error
	Close() error
}

func New(logger log.Logger, cfg config.ODFI) (Agent, error) {
	if cfg.FTP != nil {
		return newFTPTransferAgent(logger, cfg)
	}
	if cfg.SFTP != nil {
		return newSFTPTransferAgent(logger, cfg)
	}
	return nil, errors.New("upload: unknown Agent type")
}

func Type(cfg config.ODFI) string {
	if cfg.FTP != nil {
		return "ftp"
	}
	if cfg.SFTP != nil {
		return "sftp"
	}
	return "unknown"
}

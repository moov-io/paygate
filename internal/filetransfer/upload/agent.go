// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"fmt"
	"strings"

	"github.com/moov-io/paygate/internal/filetransfer/config"

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

// New returns an implementation of a Agent which is used to upload files to a remote server.
//
// This function reads ACH_FILE_TRANSFERS_ROOT_CAFILE for a file with additional root certificates to be used in all secured connections.
func New(logger log.Logger, _type string, cfg *config.Config, repo config.Repository) (Agent, error) {
	switch strings.ToLower(_type) {
	case "ftp":
		ftpConfigs, err := repo.GetFTPConfigs()
		if err != nil {
			return nil, fmt.Errorf("filetransfer: error creating new FTP client: %v", err)
		}
		return newFTPTransferAgent(logger, cfg, ftpConfigs)

	case "sftp":
		sftpConfigs, err := repo.GetSFTPConfigs()
		if err != nil {
			return nil, fmt.Errorf("filetransfer: error creating new SFTP client: %v", err)
		}
		return newSFTPTransferAgent(logger, cfg, sftpConfigs)

	default:
		return nil, fmt.Errorf("filetransfer: unknown type '%s'", _type)
	}
}

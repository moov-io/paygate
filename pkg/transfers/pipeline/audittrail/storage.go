// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package audittrail

import (
	"errors"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/config"
)

// Storage is an interface for saving and encrypting ACH files for
// records retention. This is often a requirement of agreements.
//
// File retention after upload is not part of this storage.
type Storage interface {
	// SaveFile will encrypt and copy the ACH file to the configured file storage.
	SaveFile(filename string, file *ach.File) error

	Close() error
}

func NewStorage(cfg *config.AuditTrail) (Storage, error) {
	if cfg == nil {
		return &MockStorage{}, nil
	}
	if cfg.BucketURI != "" {
		return newBlobStorage(cfg)
	}
	return nil, errors.New("unknown storage config")
}

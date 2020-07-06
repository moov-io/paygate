// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"errors"
)

type Customers struct {
	Endpoint string
	Accounts Accounts
	Debug    bool
}

func (cfg Customers) Validate() error {
	if err := cfg.Accounts.Decryptor.Validate(); err != nil {
		return err
	}
	return nil
}

type Accounts struct {
	Decryptor Decryptor
}

type Decryptor struct {
	Symmetric *Symmetric
}

func (cfg Decryptor) Validate() error {
	if cfg.Symmetric != nil && cfg.Symmetric.KeyURI == "" {
		return errors.New("symmetric: missing keyURI")
	}
	return nil
}

type Symmetric struct {
	KeyURI string
}

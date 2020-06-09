// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"errors"
)

type Customers struct {
	Endpoint string   `yaml:"endpoint" json:"endpoint"`
	Accounts Accounts `yaml:"accounts" json:"accounts"`
	Debug    bool     `yaml:"debug" json:"debug"`
}

func (cfg Customers) Validate() error {
	if err := cfg.Accounts.Decryptor.Validate(); err != nil {
		return err
	}
	return nil
}

type Accounts struct {
	Decryptor Decryptor `yaml:"decryptor" json:"decryptor"`
}

type Decryptor struct {
	Symmetric *Symmetric `yaml:"symmetric" json:"symmetric"`
}

func (cfg Decryptor) Validate() error {
	if cfg.Symmetric != nil && cfg.Symmetric.KeyURI == "" {
		return errors.New("symmetric: missing keyURI")
	}
	return nil
}

type Symmetric struct {
	KeyURI string `yaml:"keyURI" json:"keyURI"`
}

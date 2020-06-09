// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"errors"
)

type Validation struct {
	MicroDeposits *MicroDeposits `yaml:"micro_deposits" json:"micro_deposits"`
}

func (cfg Validation) Validate() error {
	if err := cfg.MicroDeposits.Validate(); err != nil {
		return err
	}
	return nil
}

type MicroDeposits struct {
	Source Source `yaml:"source" json:"source"`

	// Description is the default for what appears in the Online Banking
	// system for end-users of PayGate. Per NACHA limits this is restricted
	// to 10 characters.
	Description string `yaml:"description" json:"description"`
}

func (cfg *MicroDeposits) Validate() error {
	if cfg == nil {
		return nil
	}
	if err := cfg.Source.Validate(); err != nil {
		return err
	}
	return nil
}

type Source struct {
	CustomerID string `yaml:"customerID" json:"customerID"`
	AccountID  string `yaml:"accountID" json:"accountID"`
}

func (cfg Source) Validate() error {
	if cfg.CustomerID == "" {
		return errors.New("micro-deposits: missing Source CustomerID")
	}
	if cfg.AccountID == "" {
		return errors.New("micro-deposits: missing Source AccountID")
	}
	return nil
}

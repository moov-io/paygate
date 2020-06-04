// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

type Validation struct {
	MicroDeposits *MicroDeposits `yaml:"micro_deposits"`
}

type MicroDeposits struct {
	Source Source `yaml:"source"`

	// Description is the default for what appears in the Online Banking
	// system for end-users of PayGate. Per NACHA limits this is restricted
	// to 10 characters.
	Description string `yaml:"description"`
}

type Source struct {
	CustomerID string `yaml:"customerID"`
	AccountID  string `yaml:"accountID"`
}

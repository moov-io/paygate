// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/hash"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

type Depository struct {
	// ID is a unique string representing this Depository.
	ID id.Depository `json:"id"`

	// BankName is the legal name of the financial institution.
	BankName string `json:"bankName"`

	// Holder is the legal holder name on the account
	Holder string `json:"holder"`

	// HolderType defines the type of entity of the account holder as an individual or company
	HolderType HolderType `json:"holderType"`

	// Type defines the account as checking or savings
	Type AccountType `json:"type"`

	// RoutingNumber is the ABA routing transit number for the depository account.
	RoutingNumber string `json:"routingNumber"`

	// Status defines the current state of the Depository
	Status DepositoryStatus `json:"status"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated base.Time `json:"updated"`

	// ReturnCodes holds the optional set of return codes for why this Depository was rejected
	ReturnCodes []*ach.ReturnCode `json:"returnCodes"`

	// non-exported fields
	UserID id.User `json:"-"`

	// EncryptedAccountNumber is the account number for the depository account encrypted
	// with the attached secrets.StringKeeper
	EncryptedAccountNumber string                `json:"-"`
	HashedAccountNumber    string                `json:"-"`
	Keeper                 *secrets.StringKeeper `json:"-"`
}

func (d *Depository) Validate() error {
	if d == nil {
		return errors.New("nil Depository")
	}
	if err := d.HolderType.Validate(); err != nil {
		return err
	}
	if err := d.Type.Validate(); err != nil {
		return err
	}
	if err := d.Status.Validate(); err != nil {
		return err
	}
	if err := ach.CheckRoutingNumber(d.RoutingNumber); err != nil {
		return err
	}
	if d.EncryptedAccountNumber == "" {
		return errors.New("missing Depository.EncryptedAccountNumber")
	}
	return nil
}

func (d *Depository) ReplaceAccountNumber(num string) error {
	if d == nil || d.Keeper == nil {
		return errors.New("nil Depository and/or keeper")
	}
	encrypted, err := d.Keeper.EncryptString(num)
	if err != nil {
		return err
	}
	h, err := hash.AccountNumber(num)
	if err != nil {
		return err
	}
	d.EncryptedAccountNumber = encrypted
	d.HashedAccountNumber = h
	return nil
}

func (d *Depository) DecryptAccountNumber() (string, error) {
	if d == nil || d.Keeper == nil {
		return "", errors.New("nil Depository or keeper")
	}
	num, err := d.Keeper.DecryptString(d.EncryptedAccountNumber)
	if err != nil {
		return "", err
	}
	return num, nil
}

func (d Depository) MarshalJSON() ([]byte, error) {
	num, err := d.DecryptAccountNumber()
	if err != nil {
		return nil, err
	}
	type Aux Depository
	return json.Marshal(struct {
		Aux
		AccountNumber string `json:"accountNumber"`
	}{
		(Aux)(d),
		num,
	})
}

type HolderType string

const (
	Individual HolderType = "individual"
	Business   HolderType = "business"
)

func (t *HolderType) empty() bool {
	return string(*t) == ""
}

func (t HolderType) Validate() error {
	if t.empty() {
		return errors.New("empty HolderType")
	}
	switch t {
	case Individual, Business:
		return nil
	default:
		return fmt.Errorf("HolderType(%s) is invalid", t)
	}
}

func (t *HolderType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = HolderType(strings.ToLower(s))
	if err := t.Validate(); err != nil {
		return err
	}
	return nil
}

type DepositoryStatus string

const (
	DepositoryUnverified DepositoryStatus = "unverified"
	DepositoryVerified   DepositoryStatus = "verified"
	DepositoryRejected   DepositoryStatus = "rejected"
)

func (ds DepositoryStatus) empty() bool {
	return string(ds) == ""
}

func (ds DepositoryStatus) Validate() error {
	switch ds {
	case DepositoryUnverified, DepositoryVerified, DepositoryRejected:
		return nil
	default:
		return fmt.Errorf("DepositoryStatus(%s) is invalid", ds)
	}
}

func (ds *DepositoryStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*ds = DepositoryStatus(strings.ToLower(s))
	if err := ds.Validate(); err != nil {
		return err
	}
	return nil
}

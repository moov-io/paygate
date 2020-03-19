// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"errors"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/id"
)

type OriginatorID string

// Originator objects are an organization or person that initiates
// an ACH Transfer to a Receiver account either as a debit or credit.
// The API allows you to create, delete, and update your originators.
// You can retrieve individual originators as well as a list of all your
// originators. (Batch Header)
type Originator struct {
	// ID is a unique string representing this Originator.
	ID OriginatorID `json:"id"`

	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository id.Depository `json:"defaultDepository"`

	// Identification is a number by which the receiver is known to the originator
	// This should be the 9 digit FEIN number for a company or Social Security Number for an Individual
	Identification string `json:"identification"`

	// BirthDate is an optional value required for Know Your Customer (KYC) validation of this Originator
	BirthDate *time.Time `json:"birthDate"`

	// Address is an optional object required for Know Your Customer (KYC) validation of this Originator
	Address *Address `json:"address"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`

	// CustomerID is a unique ID that from Moov's Customers service for this Originator
	CustomerID string `json:"customerId"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated base.Time `json:"updated"`
}

func (o *Originator) missingFields() error {
	if o.DefaultDepository == "" {
		return errors.New("missing Originator.DefaultDepository")
	}
	if o.Identification == "" {
		return errors.New("missing Originator.Identification")
	}
	return nil
}

func (o *Originator) Validate() error {
	if o == nil {
		return errors.New("nil Originator")
	}
	if err := o.missingFields(); err != nil {
		return err
	}
	if o.Identification == "" {
		return errors.New("misisng Originator.Identification")
	}
	return nil
}

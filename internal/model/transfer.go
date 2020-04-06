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
	"github.com/moov-io/paygate/pkg/id"
)

type Transfer struct {
	// ID is a unique string representing this Transfer.
	ID id.Transfer `json:"id"`

	// Type determines if this is a Push or Pull transfer
	Type TransferType `json:"transferType"`

	// Amount is the country currency and quantity
	Amount Amount `json:"amount"`

	// Originator object associated with this transaction
	Originator OriginatorID `json:"originator"`

	// OriginatorDepository is the Depository associated with this transaction
	OriginatorDepository id.Depository `json:"originatorDepository"`

	// Receiver is the Receiver associated with this transaction
	Receiver ReceiverID `json:"receiver"`

	// ReceiverDepository is the id.Depository associated with this transaction
	ReceiverDepository id.Depository `json:"receiverDepository"`

	// Description is a brief summary of the transaction that may appear on the receiving entity’s financial statement
	Description string `json:"description"`

	// StandardEntryClassCode code will be generated based on Receiver type
	StandardEntryClassCode string `json:"standardEntryClassCode"`

	// Status defines the current state of the Transfer
	Status TransferStatus `json:"status"`

	// SameDay indicates that the transfer should be processed the same day if possible.
	SameDay bool `json:"sameDay"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// CCDDetail is an optional struct which enables sending CCD ACH transfers.
	CCDDetail *CCDDetail `json:"CCDDetail,omitempty"`

	// IATDetail is an optional struct which enables sending IAT ACH transfers.
	IATDetail *IATDetail `json:"IATDetail,omitempty"`

	// TELDetail is an optional struct which enables sending TEL ACH transfers.
	TELDetail *TELDetail `json:"TELDetail,omitempty"`

	// WEBDetail is an optional struct which enables sending WEB ACH transfers.
	WEBDetail *WEBDetail `json:"WEBDetail,omitempty"`

	// ReturnCode is an optional struct representing why this Transfer was returned by the RDFI
	ReturnCode *ach.ReturnCode `json:"returnCode"`

	// Hidden fields (populated in LookupTransferFromReturn) which aren't marshaled
	TransactionID string `json:"-"`
	UserID        string `json:"-"`
}

func (t *Transfer) Validate() error {
	if t == nil {
		return errors.New("nil Transfer")
	}
	if err := t.Amount.Validate(); err != nil {
		return err
	}
	if err := t.Status.Validate(); err != nil {
		return err
	}
	if t.Description == "" {
		return errors.New("transfer: missing description")
	}
	return nil
}

type TransferType string

const (
	PushTransfer TransferType = "push"
	PullTransfer TransferType = "pull"
)

func (tt TransferType) Validate() error {
	switch tt {
	case PushTransfer, PullTransfer:
		return nil
	default:
		return fmt.Errorf("TransferType(%s) is invalid", tt)
	}
}

func (tt *TransferType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*tt = TransferType(strings.ToLower(s))
	if err := tt.Validate(); err != nil {
		return err
	}
	return nil
}

type TransferStatus string

const (
	TransferCanceled   TransferStatus = "canceled"
	TransferFailed     TransferStatus = "failed"
	TransferReviewable TransferStatus = "reviewable"
	TransferPending    TransferStatus = "pending"
	TransferProcessed  TransferStatus = "processed"
)

func (ts TransferStatus) Equal(other TransferStatus) bool {
	return strings.EqualFold(string(ts), string(other))
}

func (ts TransferStatus) Validate() error {
	switch ts {
	case TransferCanceled, TransferFailed, TransferReviewable, TransferPending, TransferProcessed:
		return nil
	default:
		return fmt.Errorf("TransferStatus(%s) is invalid", ts)
	}
}

func (ts *TransferStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*ts = TransferStatus(strings.ToLower(s))
	if err := ts.Validate(); err != nil {
		return err
	}
	return nil
}

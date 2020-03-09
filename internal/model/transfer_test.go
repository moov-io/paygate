// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/id"
)

func TestTransfer__json(t *testing.T) {
	xfer := Transfer{
		ID:            id.Transfer("xfer"),
		Receiver:      ReceiverID("receiver"),
		TransactionID: "transacion",
		UserID:        "user",
		ReturnCode: &ach.ReturnCode{
			Code:   "R02",
			Reason: "Account Closed",
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&xfer); err != nil {
		t.Fatal(err)
	}

	v := buf.String()

	if !strings.Contains(v, `{"id":"xfer",`) {
		t.Error(v)
	}
	if !strings.Contains(v, `"receiver":"receiver",`) {
		t.Error(v)
	}
	if strings.Contains(v, `transaction`) {
		t.Error(v)
	}
	if strings.Contains(v, `user`) {
		t.Error(v)
	}
	if !strings.Contains(v, "R02") {
		t.Error(v)
	}
}

func TestTransfer__Validate(t *testing.T) {
	amt, _ := NewAmount("USD", "27.12")
	transfer := &Transfer{
		ID:                     id.Transfer(base.ID()),
		Type:                   PullTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
		Description:            "test transfer",
		StandardEntryClassCode: "PPD",
		Status:                 TransferPending,
	}

	if err := transfer.Validate(); err != nil {
		t.Errorf("transfer isn't valid: %v", err)
	}

	// fail due to Amount
	transfer.Amount = Amount{} // zero value
	if err := transfer.Validate(); err == nil {
		t.Error("expected error, but got none")
	}
	transfer.Amount = *amt // reset state

	// fail due to Description
	transfer.Description = ""
	if err := transfer.Validate(); err == nil {
		t.Error("expected error, but got none")
	}
}

func TestTransferType__json(t *testing.T) {
	tt := TransferType("invalid")
	valid := map[string]TransferType{
		"Pull": PullTransfer,
		"push": PushTransfer,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &tt); err != nil {
			t.Error(err.Error())
		}
		if tt != v {
			t.Errorf("got tt=%#v, v=%#v", tt, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, base.ID()))
	if err := json.Unmarshal(in, &tt); err == nil {
		t.Error("expected error")
	}
}

func TestTransferStatus__json(t *testing.T) {
	ts := TransferStatus("invalid")
	valid := map[string]TransferStatus{
		"Canceled":   TransferCanceled,
		"Failed":     TransferFailed,
		"PENDING":    TransferPending,
		"Processed":  TransferProcessed,
		"reclaimed":  TransferReclaimed,
		"reVIEWable": TransferReviewable,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &ts); err != nil {
			t.Error(err.Error())
		}
		if !ts.Equal(v) {
			t.Errorf("got ts=%#v, v=%#v", ts, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, base.ID()))
	if err := json.Unmarshal(in, &ts); err == nil {
		t.Error("expected error")
	}
}

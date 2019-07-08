// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/moov-io/base"
)

func TestWEBPaymentType(t *testing.T) {
	var paymentType WEBPaymentType
	if err := json.Unmarshal([]byte(`"SINGLE"`), &paymentType); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`"ReoCCuRRing"`), &paymentType); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`"other"`), &paymentType); err == nil {
		t.Fatal(err)
	}
}

func TestWEB__createWEBBatch(t *testing.T) {
	id, userId := base.ID(), base.ID()
	receiverDep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "foo bank",
		Holder:        "jane doe",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "121042882",
		AccountNumber: "2",
		Status:        DepositoryVerified,
		Metadata:      "jane doe checking",
	}
	receiver := &Receiver{
		ID:                ReceiverID(base.ID()),
		Email:             "jane.doe@example.com",
		DefaultDepository: receiverDep.ID,
		Status:            ReceiverVerified,
		Metadata:          "jane doe",
	}
	origDep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "foo bank",
		Holder:        "john doe",
		HolderType:    Individual,
		Type:          Savings,
		RoutingNumber: "231380104",
		AccountNumber: "2",
		Status:        DepositoryVerified,
		Metadata:      "john doe savings",
	}
	orig := &Originator{
		ID:                OriginatorID(base.ID()),
		DefaultDepository: origDep.ID,
		Identification:    "dddd",
		Metadata:          "john doe",
	}
	amt, _ := NewAmount("USD", "100.00")
	transfer := &Transfer{
		ID:                     TransferID(base.ID()),
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             orig.ID,
		OriginatorDepository:   origDep.ID,
		Receiver:               receiver.ID,
		ReceiverDepository:     receiverDep.ID,
		Description:            "sending money",
		StandardEntryClassCode: "WEB",
		Status:                 TransferPending,
		WEBDetail: WEBDetail{
			PaymentInformation: "test payment",
			PaymentType:        WEBSingle,
		},
	}

	batch, err := createWEBBatch(id, userId, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if batch == nil {
		t.Error("nil WEB Batch")
	}

	// Make sure WEBReoccurring are rejected
	transfer.WEBDetail.PaymentType = "reoccurring"
	batch, err = createWEBBatch(id, userId, transfer, receiver, receiverDep, orig, origDep)
	if batch != nil || err == nil {
		t.Errorf("expected error, but got batch: %v", batch)
	} else {
		if !strings.Contains(err.Error(), "createWEBBatch: reoccurring WEB transfers are not supported") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

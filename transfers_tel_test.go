// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package paygate

import (
	"strings"
	"testing"

	"github.com/moov-io/base"
)

func TestTEL__createTELBatch(t *testing.T) {
	id, userID := base.ID(), base.ID()
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
		StandardEntryClassCode: "TEL",
		Status:                 TransferPending,
		TELDetail: &TELDetail{
			PaymentType: "single",
		},
	}

	batch, err := createTELBatch(id, userID, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if batch == nil {
		t.Error("nil TEL Batch")
	}

	// Make sure TELReoccurring are rejected
	transfer.TELDetail.PaymentType = "reoccurring"
	batch, err = createTELBatch(id, userID, transfer, receiver, receiverDep, orig, origDep)
	if batch != nil || err == nil {
		t.Errorf("expected error, but got batch: %v", batch)
	} else {
		if !strings.Contains(err.Error(), "createTELBatch: reoccurring TEL transfers are not supported") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

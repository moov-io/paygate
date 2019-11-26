// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestCCD__createCCDBatch(t *testing.T) {
	depID, userID := base.ID(), id.User(base.ID())
	keeper := secrets.TestStringKeeper(t)

	receiverDep := &Depository{
		ID:            id.Depository(base.ID()),
		BankName:      "foo bank",
		Holder:        "jane doe",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "121042882",
		Status:        DepositoryVerified,
		Metadata:      "jane doe checking",
		keeper:        keeper,
	}
	receiverDep.ReplaceAccountNumber("2")
	receiver := &Receiver{
		ID:                ReceiverID(base.ID()),
		Email:             "jane.doe@example.com",
		DefaultDepository: receiverDep.ID,
		Status:            ReceiverVerified,
		Metadata:          "jane doe",
	}
	origDep := &Depository{
		ID:            id.Depository(base.ID()),
		BankName:      "foo bank",
		Holder:        "john doe",
		HolderType:    Individual,
		Type:          Savings,
		RoutingNumber: "231380104",
		Status:        DepositoryVerified,
		Metadata:      "john doe savings",
		keeper:        keeper,
	}
	origDep.ReplaceAccountNumber("2")
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
		StandardEntryClassCode: "CCD",
		Status:                 TransferPending,
		CCDDetail: &CCDDetail{
			PaymentInformation: "test payment",
		},
	}

	batch, err := createCCDBatch(depID, userID, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if batch == nil {
		t.Error("nil CCD Batch")
	}

	file, err := constructACHFile(depID, "", userID, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("nil CCD ach.File")
	}

	// sad path, empty CCDDetail.PaymentInformation
	transfer.CCDDetail.PaymentInformation = ""
	batch, err = createCCDBatch(depID, userID, transfer, receiver, receiverDep, orig, origDep)
	if err == nil || batch != nil {
		t.Fatalf("expected error: batch=%#v", batch)
	}
}

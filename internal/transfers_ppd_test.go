// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestPPD__createPPDBatch(t *testing.T) {
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
		StandardEntryClassCode: "PPD",
		Status:                 TransferPending,
		UserID:                 userID.String(),
	}

	batch, err := createPPDBatch(depID, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if batch == nil {
		t.Error("nil PPD Batch")
	}

	file, err := constructACHFile(depID, "", transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("nil PPD ach.File")
	}
}

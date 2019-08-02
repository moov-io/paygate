// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/moov-io/base"
)

func TestCCD__createCCDBatch(t *testing.T) {
	keeper := testSecretKeeper(testSecretKey)
	id, userId := base.ID(), base.ID()

	receiverDep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "foo bank",
		Holder:        "jane doe",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "121042882",
		Status:        DepositoryVerified,
		Metadata:      "jane doe checking",
	}
	if enc, err := encryptAccountNumber(keeper, receiverDep, "151"); err != nil {
		t.Fatal(err)
	} else {
		receiverDep.encryptedAccountNumber = enc
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
		Status:        DepositoryVerified,
		Metadata:      "john doe savings",
	}
	if enc, err := encryptAccountNumber(keeper, origDep, "143"); err != nil {
		t.Fatal(err)
	} else {
		origDep.encryptedAccountNumber = enc
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
		StandardEntryClassCode: "CCD",
		Status:                 TransferPending,
		CCDDetail: &CCDDetail{
			PaymentInformation: "test payment",
		},
	}

	batch, err := createCCDBatch(id, userId, testSecretKeeper(testSecretKey), transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if batch == nil {
		t.Error("nil CCD Batch")
	}
}

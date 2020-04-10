// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestCCD__createCCDBatch(t *testing.T) {
	depID, userID := base.ID(), id.User(base.ID())
	keeper := secrets.TestStringKeeper(t)

	gateway := &model.Gateway{
		ID:              model.GatewayID(base.ID()),
		Origin:          "987654320",
		OriginName:      "My Bank",
		Destination:     "123456780",
		DestinationName: "Their Bank",
	}
	receiverDep := &model.Depository{
		ID:            id.Depository(base.ID()),
		BankName:      "foo bank",
		Holder:        "jane doe",
		HolderType:    model.Individual,
		Type:          model.Checking,
		RoutingNumber: "121042882",
		Status:        model.DepositoryVerified,
		Metadata:      "jane doe checking",
		Keeper:        keeper,
	}
	receiverDep.ReplaceAccountNumber("2")
	receiver := &model.Receiver{
		ID:                model.ReceiverID(base.ID()),
		Email:             "jane.doe@example.com",
		DefaultDepository: receiverDep.ID,
		Status:            model.ReceiverVerified,
		Metadata:          "jane doe",
	}
	origDep := &model.Depository{
		ID:            id.Depository(base.ID()),
		BankName:      "foo bank",
		Holder:        "john doe",
		HolderType:    model.Individual,
		Type:          model.Savings,
		RoutingNumber: "231380104",
		Status:        model.DepositoryVerified,
		Metadata:      "john doe savings",
		Keeper:        keeper,
	}
	origDep.ReplaceAccountNumber("2")
	orig := &model.Originator{
		ID:                model.OriginatorID(base.ID()),
		DefaultDepository: origDep.ID,
		Identification:    "dddd",
		Metadata:          "john doe",
	}
	amt, _ := model.NewAmount("USD", "100.00")
	transfer := &model.Transfer{
		ID:                     id.Transfer(base.ID()),
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             orig.ID,
		OriginatorDepository:   origDep.ID,
		Receiver:               receiver.ID,
		ReceiverDepository:     receiverDep.ID,
		Description:            "sending money",
		StandardEntryClassCode: "CCD",
		Status:                 model.TransferPending,
		UserID:                 userID.String(),
		CCDDetail: &model.CCDDetail{
			PaymentInformation: "test payment",
		},
	}

	batch, err := createCCDBatch(depID, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if batch == nil {
		t.Error("nil CCD Batch")
	}

	file, err := ConstructFile(depID, gateway, transfer, orig, origDep, receiver, receiverDep)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("nil CCD ach.File")
	}

	// sad path, empty CCDDetail.PaymentInformation
	transfer.CCDDetail.PaymentInformation = ""
	batch, err = createCCDBatch(depID, transfer, receiver, receiverDep, orig, origDep)
	if err == nil || batch != nil {
		t.Fatalf("expected error: batch=%#v", batch)
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestTELPaymentType(t *testing.T) {
	var paymentType model.TELPaymentType
	if err := json.Unmarshal([]byte(`"SINGLE"`), &paymentType); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`"ReoCCuRRing"`), &paymentType); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`"other"`), &paymentType); err == nil {
		t.Fatal("expected error")
	}
	if err := json.Unmarshal([]byte("1"), &paymentType); err == nil {
		t.Fatal("expected error")
	}
}

func TestTEL__createTELBatch(t *testing.T) {
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
		ID:                     id.Depository(base.ID()),
		BankName:               "foo bank",
		Holder:                 "john doe",
		HolderType:             model.Individual,
		Type:                   model.Savings,
		RoutingNumber:          "231380104",
		EncryptedAccountNumber: "2",
		Status:                 model.DepositoryVerified,
		Metadata:               "john doe savings",
		Keeper:                 keeper,
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
		StandardEntryClassCode: "TEL",
		Status:                 model.TransferPending,
		UserID:                 userID.String(),
		TELDetail: &model.TELDetail{
			PaymentType: "single",
		},
	}

	batch, err := createTELBatch(depID, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if batch == nil {
		t.Error("nil TEL Batch")
	}

	file, err := ConstructFile(depID, "", gateway, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("nil TEL ach.File")
	}

	// Make sure TELReoccurring are rejected
	transfer.TELDetail.PaymentType = "reoccurring"
	batch, err = createTELBatch(depID, transfer, receiver, receiverDep, orig, origDep)
	if batch != nil || err == nil {
		t.Errorf("expected error, but got batch: %v", batch)
	} else {
		if !strings.Contains(err.Error(), "createTELBatch: reoccurring TEL transfers are not supported") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

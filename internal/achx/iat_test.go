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

func TestIAT__Validate(t *testing.T) {
	iat := model.IATDetail{}
	if err := iat.Validate(); err == nil {
		t.Error("expected error")
	}
	iat.OriginatorName = "a"
	iat.OriginatorAddress = "aa"
	iat.OriginatorCity = "aaa"
	iat.OriginatorState = "bb"
	iat.OriginatorPostalCode = "bbb"
	iat.OriginatorCountryCode = "ccc"
	if err := iat.Validate(); err == nil {
		t.Error("expected error")
	}
	iat.ODFIName = "b"
	iat.ODFIIDNumberQualifier = "01"
	iat.ODFIIdentification = "b"
	iat.ODFIBranchCurrencyCode = "b"
	if err := iat.Validate(); err == nil {
		t.Error("expected error")
	}
	iat.ReceiverName = "c"
	iat.ReceiverAddress = "c"
	iat.ReceiverCity = "c"
	iat.ReceiverState = "c"
	iat.ReceiverPostalCode = "c"
	iat.ReceiverCountryCode = "c"
	if err := iat.Validate(); err == nil {
		t.Error("expected error")
	}
	iat.RDFIName = "d"
	iat.RDFIIDNumberQualifier = "01"
	iat.RDFIIdentification = "d"
	iat.RDFIBranchCurrencyCode = "d"
	if err := iat.Validate(); err == nil {
		t.Error("expected error")
	}
	iat.ForeignCorrespondentBankName = "d"
	iat.ForeignCorrespondentBankIDNumberQualifier = "d"
	iat.ForeignCorrespondentBankIDNumber = "d"
	iat.ForeignCorrespondentBankBranchCountryCode = "d"
	if err := iat.Validate(); err != nil {
		t.Errorf("expected no error: %v", err)
	}
}

func TestIAT__createIATBatch(t *testing.T) {
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
		StandardEntryClassCode: "IAT",
		Status:                 model.TransferPending,
		UserID:                 userID.String(),
		IATDetail: &model.IATDetail{
			OriginatorName:               orig.Metadata,
			OriginatorAddress:            "123 1st st",
			OriginatorCity:               "anytown",
			OriginatorState:              "PA",
			OriginatorPostalCode:         "12345",
			OriginatorCountryCode:        "US",
			ODFIName:                     "my bank",
			ODFIIDNumberQualifier:        "01",
			ODFIIdentification:           "2",
			ODFIBranchCurrencyCode:       "USD",
			ReceiverName:                 receiver.Metadata,
			ReceiverAddress:              "321 2nd st",
			ReceiverCity:                 "othertown",
			ReceiverState:                "GB",
			ReceiverPostalCode:           "54321",
			ReceiverCountryCode:          "GB",
			RDFIName:                     "their bank",
			RDFIIDNumberQualifier:        "01",
			RDFIIdentification:           "4",
			RDFIBranchCurrencyCode:       "GBP",
			ForeignCorrespondentBankName: "their bank",
			ForeignCorrespondentBankIDNumberQualifier: "5",
			ForeignCorrespondentBankIDNumber:          "6",
			ForeignCorrespondentBankBranchCountryCode: "GB",
		},
	}

	batch, err := createIATBatch(depID, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if batch == nil {
		t.Error("nil IAT Batch")
	}

	file, err := ConstructFile(depID, "", gateway, transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("nil IAT ach.File")
	}
}

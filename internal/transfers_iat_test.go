// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestIAT__validate(t *testing.T) {
	iat := IATDetail{}
	if err := iat.validate(); err == nil {
		t.Error("expected error")
	}
	iat.OriginatorName = "a"
	iat.OriginatorAddress = "aa"
	iat.OriginatorCity = "aaa"
	iat.OriginatorState = "bb"
	iat.OriginatorPostalCode = "bbb"
	iat.OriginatorCountryCode = "ccc"
	if err := iat.validate(); err == nil {
		t.Error("expected error")
	}
	iat.ODFIName = "b"
	iat.ODFIIDNumberQualifier = "01"
	iat.ODFIIdentification = "b"
	iat.ODFIBranchCurrencyCode = "b"
	if err := iat.validate(); err == nil {
		t.Error("expected error")
	}
	iat.ReceiverName = "c"
	iat.ReceiverAddress = "c"
	iat.ReceiverCity = "c"
	iat.ReceiverState = "c"
	iat.ReceiverPostalCode = "c"
	iat.ReceiverCountryCode = "c"
	if err := iat.validate(); err == nil {
		t.Error("expected error")
	}
	iat.RDFIName = "d"
	iat.RDFIIDNumberQualifier = "01"
	iat.RDFIIdentification = "d"
	iat.RDFIBranchCurrencyCode = "d"
	if err := iat.validate(); err == nil {
		t.Error("expected error")
	}
	iat.ForeignCorrespondentBankName = "d"
	iat.ForeignCorrespondentBankIDNumberQualifier = "d"
	iat.ForeignCorrespondentBankIDNumber = "d"
	iat.ForeignCorrespondentBankBranchCountryCode = "d"
	if err := iat.validate(); err != nil {
		t.Errorf("expected no error: %v", err)
	}
}

func TestIAT__createIATBatch(t *testing.T) {
	depID, userID := base.ID(), id.User(base.ID())
	keeper := secrets.TestStringKeeper(t)

	receiverDep := &Depository{
		ID:            id.Depository(base.ID()),
		BankName:      "foo bank",
		Holder:        "jane doe",
		HolderType:    Individual,
		Type:          model.Checking,
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
		Type:          model.Savings,
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
	amt, _ := model.NewAmount("USD", "100.00")
	transfer := &Transfer{
		ID:                     TransferID(base.ID()),
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             orig.ID,
		OriginatorDepository:   origDep.ID,
		Receiver:               receiver.ID,
		ReceiverDepository:     receiverDep.ID,
		Description:            "sending money",
		StandardEntryClassCode: "IAT",
		Status:                 TransferPending,
		UserID:                 userID.String(),
		IATDetail: &IATDetail{
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

	file, err := constructACHFile(depID, "", transfer, receiver, receiverDep, orig, origDep)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("nil IAT ach.File")
	}
}

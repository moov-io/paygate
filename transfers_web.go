// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
)

type WEBDetail struct {
	PaymentInformation string         `json:"paymentInformation,omitempty"`
	PaymentType        WEBPaymentType `json:"paymentType,omitempty"`
}

type WEBPaymentType string

const (
	WEBSingle      WEBPaymentType = "single"
	WEBReoccurring WEBPaymentType = "reoccurring"
)

func (t WEBPaymentType) validate() error {
	switch t {
	case WEBSingle, WEBReoccurring:
		return nil
	default:
		return fmt.Errorf("WEBPaymentType(%s) is invalid", t)
	}
}

func (t *WEBPaymentType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = WEBPaymentType(strings.ToLower(s))
	if err := t.validate(); err != nil {
		return err
	}
	return nil
}

func createWEBBatch(id, userId string, transfer *Transfer, receiver *Receiver, receiverDep *Depository, orig *Originator, origDep *Depository) (ach.Batcher, error) {
	batchHeader := ach.NewBatchHeader()
	batchHeader.ID = id
	batchHeader.ServiceClassCode = determineServiceClassCode(transfer)
	batchHeader.CompanyName = orig.Metadata
	batchHeader.StandardEntryClassCode = ach.WEB
	batchHeader.CompanyIdentification = orig.Identification
	batchHeader.CompanyEntryDescription = transfer.Description
	batchHeader.EffectiveEntryDate = base.Now().AddBankingDay(1).Format("060102") // Date to be posted, YYMMDD
	batchHeader.ODFIIdentification = aba8(origDep.RoutingNumber)

	// Add EntryDetail to WEB batch
	entryDetail := ach.NewEntryDetail()
	entryDetail.ID = id
	entryDetail.TransactionCode = determineTransactionCode(transfer)
	entryDetail.RDFIIdentification = aba8(receiverDep.RoutingNumber)
	entryDetail.CheckDigit = abaCheckDigit(receiverDep.RoutingNumber)
	entryDetail.DFIAccountNumber = receiverDep.AccountNumber
	entryDetail.Amount = transfer.Amount.Int()
	entryDetail.IdentificationNumber = createIdentificationNumber()
	entryDetail.IndividualName = receiver.Metadata
	entryDetail.TraceNumber = createTraceNumber(origDep.RoutingNumber)

	// WEB transfers use DiscretionaryData for PaymentTypeCode
	if transfer.WEBDetail.PaymentType == WEBSingle {
		entryDetail.DiscretionaryData = "S"
	} else {
		entryDetail.DiscretionaryData = "R"
	}

	// Add Addenda05
	addenda05 := ach.NewAddenda05()
	addenda05.ID = id
	addenda05.PaymentRelatedInformation = transfer.WEBDetail.PaymentInformation
	addenda05.SequenceNumber = 1
	addenda05.EntryDetailSequenceNumber = 1
	entryDetail.AddAddenda05(addenda05)
	entryDetail.AddendaRecordIndicator = 1

	// For now just create WEB
	batch, err := ach.NewBatch(batchHeader)
	if err != nil {
		return nil, fmt.Errorf("ACH file %s (userId=%s): failed to create batch: %v", id, userId, err)
	}
	batch.AddEntry(entryDetail)
	batch.SetControl(ach.NewBatchControl())

	if err := batch.Create(); err != nil {
		return batch, err
	}
	return batch, nil
}

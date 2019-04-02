// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
)

func createPPDBatch(id, userId string, transfer *Transfer, cust *Customer, custDep *Depository, orig *Originator) (ach.Batcher, error) {
	batchHeader := ach.NewBatchHeader()
	batchHeader.ID = id
	batchHeader.ServiceClassCode = 220 // Credits: 220, Debits: 225
	batchHeader.CompanyName = orig.Metadata
	if batchHeader.CompanyName == "" {
		batchHeader.CompanyName = "Moov - Paygate payment" // TODO(adam)
	}

	batchHeader.StandardEntryClassCode = strings.ToUpper(transfer.StandardEntryClassCode)
	batchHeader.CompanyIdentification = "121042882" // 9 digit FEIN number
	batchHeader.CompanyEntryDescription = transfer.Description
	batchHeader.EffectiveEntryDate = base.Now().AddBankingDay(1).Format("060102") // Date to be posted, YYMMDD
	batchHeader.ODFIIdentification = orig.Identification

	// Add EntryDetail to PPD batch
	entryDetail := ach.NewEntryDetail()
	entryDetail.ID = id
	// Credit (deposit) to checking account ‘22’
	// Prenote for credit to checking account ‘23’
	// Debit (withdrawal) to checking account ‘27’
	// Prenote for debit to checking account ‘28’
	// Credit to savings account ‘32’
	// Prenote for credit to savings account ‘33’
	// Debit to savings account ‘37’
	// Prenote for debit to savings account ‘38’
	// TODO(adam): exported const's for use
	entryDetail.TransactionCode = 22
	entryDetail.RDFIIdentification = aba8(custDep.RoutingNumber)
	entryDetail.CheckDigit = abaCheckDigit(custDep.RoutingNumber)
	entryDetail.DFIAccountNumber = custDep.AccountNumber
	entryDetail.Amount = transfer.Amount.Int()
	entryDetail.IdentificationNumber = "#83738AB#      " // internal identification (alphanumeric)
	entryDetail.IndividualName = cust.Metadata           // TODO(adam): and/or custDep.Metadata ?
	entryDetail.DiscretionaryData = transfer.Description
	entryDetail.TraceNumber = "121042880000001" // TODO(adam): assigned by ODFI // 0-9 of x-idempotency-key ?

	// Add Addenda05
	addenda05 := ach.NewAddenda05()
	addenda05.ID = id
	addenda05.PaymentRelatedInformation = "paygate transaction"
	addenda05.SequenceNumber = 1
	addenda05.EntryDetailSequenceNumber = 1
	entryDetail.AddAddenda05(addenda05)
	entryDetail.AddendaRecordIndicator = 1

	// For now just create PPD
	batch, err := ach.NewBatch(batchHeader)
	if err != nil {
		return nil, fmt.Errorf("ACH file %s (userId=%s): failed to create batch: %v", id, userId, err)
	}
	batch.AddEntry(entryDetail)
	batch.SetControl(ach.NewBatchControl())
	return batch, nil
}

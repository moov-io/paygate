// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package remoteach

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/achx"
	"github.com/moov-io/paygate/internal/model"
)

func createPPDBatch(id string, transfer *model.Transfer, receiver *model.Receiver, receiverDep *model.Depository, orig *model.Originator, origDep *model.Depository) (ach.Batcher, error) {
	batchHeader := makeBatchHeader(id, transfer, orig, origDep)
	batchHeader.StandardEntryClassCode = ach.PPD

	// Add EntryDetail to PPD batch
	entryDetail := ach.NewEntryDetail()
	entryDetail.ID = id
	entryDetail.TransactionCode = determineTransactionCode(transfer, origDep)
	entryDetail.RDFIIdentification = achx.ABA8(receiverDep.RoutingNumber)
	entryDetail.CheckDigit = achx.ABACheckDigit(receiverDep.RoutingNumber)
	entryDetail.Amount = transfer.Amount.Int()
	entryDetail.IdentificationNumber = createIdentificationNumber()
	entryDetail.IndividualName = receiver.Metadata
	entryDetail.DiscretionaryData = transfer.Description
	entryDetail.TraceNumber = achx.TraceNumber(origDep.RoutingNumber)

	if num, err := receiverDep.DecryptAccountNumber(); err != nil {
		return nil, fmt.Errorf("PPD: receiver account number decrypt failed: %v", err)
	} else {
		entryDetail.DFIAccountNumber = num
	}

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
		return nil, fmt.Errorf("failed to create PPD batch: %v", err)
	}
	batch.AddEntry(entryDetail)
	batch.SetControl(ach.NewBatchControl())

	if err := batch.Create(); err != nil {
		return batch, err
	}
	return batch, nil
}

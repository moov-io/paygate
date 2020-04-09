// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"errors"
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/model"
)

func createWEBBatch(id string, transfer *model.Transfer, receiver *model.Receiver, receiverDep *model.Depository, orig *model.Originator, origDep *model.Depository) (ach.Batcher, error) {
	batchHeader := makeBatchHeader(id, transfer, orig, origDep)
	batchHeader.StandardEntryClassCode = ach.WEB

	// Add EntryDetail to WEB batch
	entryDetail := ach.NewEntryDetail()
	entryDetail.ID = id
	entryDetail.TransactionCode = determineTransactionCode(transfer, origDep)
	entryDetail.RDFIIdentification = ABA8(receiverDep.RoutingNumber)
	entryDetail.CheckDigit = ABACheckDigit(receiverDep.RoutingNumber)
	entryDetail.Amount = transfer.Amount.Int()
	entryDetail.IdentificationNumber = createIdentificationNumber()
	entryDetail.IndividualName = receiver.Metadata
	entryDetail.TraceNumber = TraceNumber(origDep.RoutingNumber)

	if num, err := receiverDep.DecryptAccountNumber(); err != nil {
		return nil, fmt.Errorf("WEB: receiver account number decrypt failed: %v", err)
	} else {
		entryDetail.DFIAccountNumber = num
	}

	if transfer.WEBDetail == nil {
		return nil, errors.New("nil WEB detail")
	}

	// WEB transfers use DiscretionaryData for PaymentTypeCode
	if transfer.WEBDetail.PaymentType == model.WEBSingle {
		entryDetail.DiscretionaryData = "S"
	} else {
		entryDetail.DiscretionaryData = "R"
		return nil, fmt.Errorf("createWEBBatch: %s WEB transfers are not supported yet", model.WEBReoccurring)
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
		return nil, fmt.Errorf("failed to create WEB batch: %v", err)
	}
	batch.AddEntry(entryDetail)
	batch.SetControl(ach.NewBatchControl())

	if err := batch.Create(); err != nil {
		return batch, err
	}
	return batch, nil
}

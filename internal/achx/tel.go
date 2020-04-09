// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/model"
)

// createTELBatch creates and returns a TEL ACH batch for use after receiving oral authorization to debit a customer's account.
//
// TEL batches require a telephone number that's answered during typical business hours along with a date and statement of oral
// authorization for a one-time funds transfer. Recurring transfers must contain the total amount of transfers or conditions for
// scheduling transfers. Originators must retain written notice of the authorization for two years.
func createTELBatch(id string, transfer *model.Transfer, receiver *model.Receiver, receiverDep *model.Depository, orig *model.Originator, origDep *model.Depository) (ach.Batcher, error) {
	batchHeader := makeBatchHeader(id, transfer, orig, origDep)
	batchHeader.StandardEntryClassCode = ach.TEL

	// Add EntryDetail to PPD batch
	entryDetail := ach.NewEntryDetail()
	entryDetail.ID = id
	entryDetail.TransactionCode = determineTransactionCode(transfer, origDep)
	entryDetail.RDFIIdentification = ABA8(receiverDep.RoutingNumber)
	entryDetail.CheckDigit = ABACheckDigit(receiverDep.RoutingNumber)
	entryDetail.Amount = transfer.Amount.Int()
	if transfer.Description != "" {
		r := strings.NewReplacer("-", "", ".", "", " ", "")
		entryDetail.IdentificationNumber = r.Replace(transfer.Description) // phone number (which TEL requires)
	} else {
		entryDetail.IdentificationNumber = createIdentificationNumber()
	}
	entryDetail.IndividualName = receiver.Metadata
	entryDetail.TraceNumber = TraceNumber(origDep.RoutingNumber)

	if num, err := receiverDep.DecryptAccountNumber(); err != nil {
		return nil, fmt.Errorf("TEL: receiver account number decrypt failed: %v", err)
	} else {
		entryDetail.DFIAccountNumber = num
	}

	if transfer.TELDetail == nil {
		return nil, errors.New("nil TEL detail")
	}

	// TEL transfers use DiscretionaryData for PaymentTypeCode
	if transfer.TELDetail.PaymentType == model.TELSingle {
		entryDetail.DiscretionaryData = "S"
	} else {
		entryDetail.DiscretionaryData = "R"
		return nil, fmt.Errorf("createTELBatch: %s TEL transfers are not supported yet", model.TELReoccurring)
	}

	// For now just create PPD
	batch, err := ach.NewBatch(batchHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create TEL batch: %v", err)
	}
	batch.AddEntry(entryDetail)
	batch.SetControl(ach.NewBatchControl())

	if err := batch.Create(); err != nil {
		return batch, err
	}
	return batch, nil
}

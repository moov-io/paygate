// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

func createPPDBatch(id string, odfi config.ODFI, xfer *client.Transfer, source Source, destination Destination) (ach.Batcher, error) {
	bh := makeBatchHeader(id, odfi, xfer, source.Account)
	bh.StandardEntryClassCode = ach.PPD

	// Add EntryDetail to PPD batch
	ed := ach.NewEntryDetail()
	ed.ID = id
	ed.TransactionCode = 27
	// ed.TransactionCode = determineTransactionCode(transfer, origDep)
	ed.RDFIIdentification = ABA8(destination.Account.RoutingNumber)
	ed.CheckDigit = ABACheckDigit(destination.Account.RoutingNumber)
	ed.Amount = 100 // xfer.Amount.Int() // TODO(adam): impl
	ed.IdentificationNumber = createIdentificationNumber()
	ed.IndividualName = fmt.Sprintf("%s %s", destination.Customer.FirstName, destination.Customer.LastName)
	ed.DiscretionaryData = xfer.Description
	ed.TraceNumber = TraceNumber(source.Account.RoutingNumber)

	// TODO(adam): need to decrypt
	// ed.DFIAccountNumber = num
	ed.DFIAccountNumber = ""

	// Add Addenda05
	addenda05 := ach.NewAddenda05()
	addenda05.ID = id
	addenda05.PaymentRelatedInformation = xfer.Description
	addenda05.SequenceNumber = 1
	addenda05.EntryDetailSequenceNumber = 1
	ed.AddAddenda05(addenda05)
	ed.AddendaRecordIndicator = 1

	// For now just create PPD
	batch, err := ach.NewBatch(bh)
	if err != nil {
		return nil, fmt.Errorf("failed to create PPD batch: %v", err)
	}
	batch.AddEntry(ed)
	batch.SetControl(ach.NewBatchControl())

	if err := batch.Create(); err != nil {
		return batch, err
	}
	return batch, nil
}

// 	if num, err := receiverDep.DecryptAccountNumber(); err != nil {
// 		return nil, fmt.Errorf("PPD: receiver account number decrypt failed: %v", err)
// 	}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/model"
)

func createCCDBatch(id string, options Options, xfer *client.Transfer, source Source, destination Destination) (ach.Batcher, error) {
	bh := makeBatchHeader(id, options, xfer, source)
	bh.StandardEntryClassCode = ach.CCD

	var amt model.Amount
	if err := amt.FromString(xfer.Amount); err != nil {
		return nil, fmt.Errorf("unable to parse '%s': %v", xfer.Amount, err)
	}

	// Create CCD batch
	batch, err := ach.NewBatch(bh)
	if err != nil {
		return nil, fmt.Errorf("failed to create CCD batch: %v", err)
	}

	entry := createCCDEntry(id, options, xfer, amt, source, destination)
	batch.AddEntry(entry)

	if options.FileConfig.BalanceEntries {
		balance, err := balanceEntry(entry, options, source, destination)
		if err != nil {
			return nil, fmt.Errorf("problem balancing entry: %#v", err)
		}
		batch.AddEntry(balance)
	}

	batch.SetControl(ach.NewBatchControl())

	if err := batch.Create(); err != nil {
		return batch, err
	}
	return batch, nil
}

func createCCDEntry(id string, options Options, xfer *client.Transfer, amt model.Amount, src Source, dst Destination) *ach.EntryDetail {
	ed := ach.NewEntryDetail()
	ed.ID = id

	// Set the fields which are the same across debits and credits
	ed.Amount = amt.Int()
	ed.IdentificationNumber = createIdentificationNumber()
	ed.DiscretionaryData = xfer.Description
	ed.TraceNumber = TraceNumber(options.ODFIRoutingNumber)
	ed.Category = ach.CategoryForward

	// Set fields based on which FI is getting the funds
	ed.TransactionCode = determineTransactionCode(options, src.Account)
	if options.ODFIRoutingNumber == src.Account.RoutingNumber {
		// Credit
		ed.RDFIIdentification = ABA8(dst.Account.RoutingNumber)
		ed.CheckDigit = ABACheckDigit(dst.Account.RoutingNumber)
		ed.DFIAccountNumber = dst.AccountNumber
		ed.IndividualName = fmt.Sprintf("%s %s", dst.Customer.FirstName, dst.Customer.LastName)
	} else {
		// Debit
		ed.RDFIIdentification = ABA8(src.Account.RoutingNumber)
		ed.CheckDigit = ABACheckDigit(src.Account.RoutingNumber)
		ed.DFIAccountNumber = src.AccountNumber
		ed.IndividualName = fmt.Sprintf("%s %s", src.Customer.FirstName, src.Customer.LastName)
	}

	// Add the Addenda05 record if we're configured to do so
	if options.FileConfig.Addendum.Create05 {
		ed.AddendaRecordIndicator = 1

		addenda05 := ach.NewAddenda05()
		addenda05.ID = id
		addenda05.PaymentRelatedInformation = xfer.Description
		addenda05.SequenceNumber = 1
		addenda05.EntryDetailSequenceNumber = 1

		ed.AddAddenda05(addenda05)
	}

	return ed
}

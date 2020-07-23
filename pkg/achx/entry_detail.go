// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"fmt"
	"strconv"

	"github.com/moov-io/ach"
	customers "github.com/moov-io/customers/client"
)

func determineTransactionCode(options Options, srcAcct customers.Account) int {
	if options.ODFIRoutingNumber == srcAcct.RoutingNumber {
		// Credit
		if srcAcct.Type == customers.CHECKING {
			return ach.CheckingCredit
		}
		if srcAcct.Type == customers.SAVINGS {
			return ach.SavingsCredit
		}
	}
	// Debit
	if srcAcct.Type == customers.CHECKING {
		return ach.CheckingDebit
	}
	if srcAcct.Type == customers.SAVINGS {
		return ach.SavingsDebit
	}
	return 0 // invalid, represents a logic bug
}

func balanceEntry(entry *ach.EntryDetail, options Options, src Source, dst Destination) (*ach.EntryDetail, error) {
	ed := ach.NewEntryDetail()
	ed.ID = entry.ID

	// Set the fields which are the same across debits and credits
	ed.Amount = entry.Amount
	ed.IdentificationNumber = createIdentificationNumber()
	ed.DiscretionaryData = "OFFSET"
	ed.Category = ach.CategoryForward

	trace, err := strconv.ParseInt(entry.TraceNumber, 10, 64)
	if err != nil {
		return nil, err
	}
	ed.TraceNumber = fmt.Sprintf("%d", trace+1)

	// Set fields based on which FI is getting the funds
	ed.TransactionCode = determineTransactionCode(options, dst.Account)

	if options.ODFIRoutingNumber == src.Account.RoutingNumber {
		// Credit
		ed.RDFIIdentification = ABA8(src.Account.RoutingNumber)
		ed.CheckDigit = ABACheckDigit(src.Account.RoutingNumber)
		ed.DFIAccountNumber = src.AccountNumber
		ed.IndividualName = fmt.Sprintf("%s %s", src.Customer.FirstName, src.Customer.LastName)
	} else {
		// Debit
		ed.RDFIIdentification = ABA8(dst.Account.RoutingNumber)
		ed.CheckDigit = ABACheckDigit(dst.Account.RoutingNumber)
		ed.DFIAccountNumber = dst.AccountNumber
		ed.IndividualName = fmt.Sprintf("%s %s", dst.Customer.FirstName, dst.Customer.LastName)
	}

	// Add the Addenda05 record if we're configured to do so
	if options.FileConfig.Addendum.Create05 {
		ed.AddendaRecordIndicator = 1

		addenda05 := ach.NewAddenda05()
		addenda05.ID = entry.ID
		addenda05.PaymentRelatedInformation = "OFFSET"
		addenda05.SequenceNumber = 1
		addenda05.EntryDetailSequenceNumber = 1

		ed.AddAddenda05(addenda05)
	}

	return ed, nil
}

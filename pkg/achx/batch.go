// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	customers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

// makeBatchHeader ...
// This method does not set the StandardEntryClassCode
// TODO(adam): write docs
func makeBatchHeader(id string, odfi config.ODFI, xfer client.Transfer, sourceAccount customers.Account) *ach.BatchHeader {
	batchHeader := ach.NewBatchHeader()
	batchHeader.ID = id

	// Picking between credit and debit is based on which of a transfer's source or destination is the ODFI.
	if odfi.RoutingNumber == sourceAccount.RoutingNumber {
		batchHeader.ServiceClassCode = ach.CreditsOnly
	} else {
		batchHeader.ServiceClassCode = ach.DebitsOnly
	}

	batchHeader.CompanyName = "test"              // TODO(adam): impl from Customers -- was origDep.Holder
	batchHeader.CompanyDiscretionaryData = "test" // TODO(adam): impl customer.Metadata
	batchHeader.CompanyIdentification = "test"    // TODO(adam): impl orig.Identification
	batchHeader.CompanyEntryDescription = xfer.Description
	batchHeader.CompanyDescriptiveDate = time.Now().Format("060102")
	batchHeader.EffectiveEntryDate = base.Now().AddBankingDay(1).Format("060102") // Date to be posted, YYMMDD
	batchHeader.ODFIIdentification = ABA8("987654320")                            // TODO(adam): impl was origDep.RoutingNumber

	return batchHeader
}

func createIdentificationNumber() string {
	return base.ID()[:15]
}

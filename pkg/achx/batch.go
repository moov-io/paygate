// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"fmt"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
)

// makeBatchHeader creates an ach.BatchHeader from the given Transfer and source Account.
//
// This method does not set the StandardEntryClassCode.
func makeBatchHeader(id string, options Options, companyID string, xfer *client.Transfer, source Source) *ach.BatchHeader {
	batchHeader := ach.NewBatchHeader()
	batchHeader.ID = id

	// Picking between credit and debit is based on which of a transfer's source or destination is the ODFI.
	if options.FileConfig.OffsetEntries {
		batchHeader.ServiceClassCode = ach.MixedDebitsAndCredits
	} else {
		if options.ODFIRoutingNumber == source.Account.RoutingNumber {
			batchHeader.ServiceClassCode = ach.CreditsOnly
		} else {
			batchHeader.ServiceClassCode = ach.DebitsOnly
		}
	}

	// Set the Company Name from Customer information
	batchHeader.CompanyName = fmt.Sprintf("%s %s", source.Customer.FirstName, source.Customer.LastName)
	if source.Customer.NickName != "" {
		batchHeader.CompanyName = source.Customer.NickName
	}

	// Set DiscretionaryData if it exists
	if v, ok := source.Customer.Metadata["discretionary"]; ok {
		batchHeader.CompanyDiscretionaryData = v
	}

	// Fill in the other fields
	batchHeader.CompanyIdentification = companyID // from client.Tenant
	batchHeader.CompanyEntryDescription = xfer.Description
	batchHeader.CompanyDescriptiveDate = time.Now().Format("060102")
	batchHeader.EffectiveEntryDate = base.Now().AddBankingDay(1).Format("060102") // Date to be posted, YYMMDD
	batchHeader.ODFIIdentification = ABA8(options.ODFIRoutingNumber)

	return batchHeader
}

func createIdentificationNumber() string {
	return base.ID()[:15]
}

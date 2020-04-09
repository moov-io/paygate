// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
)

// makeBatchHeader ...
// This method does not set the StandardEntryClassCode
func makeBatchHeader(id string, transfer *model.Transfer, orig *model.Originator, origDep *model.Depository) *ach.BatchHeader {
	batchHeader := ach.NewBatchHeader()
	batchHeader.ID = id
	batchHeader.ServiceClassCode = determineServiceClassCode(transfer)
	batchHeader.CompanyName = origDep.Holder
	batchHeader.CompanyDiscretionaryData = orig.Metadata
	batchHeader.CompanyIdentification = orig.Identification
	batchHeader.CompanyEntryDescription = transfer.Description
	batchHeader.CompanyDescriptiveDate = time.Now().Format("060102")
	batchHeader.EffectiveEntryDate = base.Now().AddBankingDay(1).Format("060102") // Date to be posted, YYMMDD
	batchHeader.ODFIIdentification = ABA8(origDep.RoutingNumber)
	return batchHeader
}

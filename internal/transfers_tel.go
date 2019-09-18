// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
)

type TELDetail struct {
	PhoneNumber string         `json:"phoneNumber"`
	PaymentType TELPaymentType `json:"paymentType,omitempty"`
}

type TELPaymentType string

const (
	TELSingle      TELPaymentType = "single"
	TELReoccurring TELPaymentType = "reoccurring"
)

func (t TELPaymentType) validate() error {
	switch t {
	case TELSingle, TELReoccurring:
		return nil
	default:
		return fmt.Errorf("TELPaymentType(%s) is invalid", t)
	}
}

func (t *TELPaymentType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = TELPaymentType(strings.ToLower(s))
	if err := t.validate(); err != nil {
		return err
	}
	return nil
}

// createTELBatch creates and returns a TEL ACH batch for use after receiving oral authorization to debit a customer's account.
//
// TEL batches require a telephone number that's answered during typical business hours along with a date and statement of oral
// authorization for a one-time funds transfer. Recurring transfers must contain the total amount of transfers or conditions for
// scheduling transfers. Originators must retain written notice of the authorization for two years.
func createTELBatch(id, userId string, transfer *Transfer, receiver *Receiver, receiverDep *Depository, orig *Originator, origDep *Depository) (ach.Batcher, error) {
	batchHeader := ach.NewBatchHeader()
	batchHeader.ID = id
	batchHeader.ServiceClassCode = ach.DebitsOnly
	batchHeader.CompanyName = orig.Metadata
	batchHeader.StandardEntryClassCode = ach.TEL
	batchHeader.CompanyIdentification = orig.Identification
	batchHeader.CompanyEntryDescription = transfer.Description
	batchHeader.CompanyDescriptiveDate = time.Now().Format("060102")
	batchHeader.EffectiveEntryDate = base.Now().AddBankingDay(1).Format("060102") // Date to be posted, YYMMDD
	batchHeader.ODFIIdentification = aba8(origDep.RoutingNumber)

	// Add EntryDetail to PPD batch
	entryDetail := ach.NewEntryDetail()
	entryDetail.ID = id
	entryDetail.TransactionCode = determineTransactionCode(transfer, origDep)
	entryDetail.RDFIIdentification = aba8(receiverDep.RoutingNumber)
	entryDetail.CheckDigit = abaCheckDigit(receiverDep.RoutingNumber)
	entryDetail.DFIAccountNumber = receiverDep.AccountNumber
	entryDetail.Amount = transfer.Amount.Int()
	if transfer.Description != "" {
		r := strings.NewReplacer("-", "", ".", "", " ", "")
		entryDetail.IdentificationNumber = r.Replace(transfer.Description) // phone number (which TEL requires)
	} else {
		entryDetail.IdentificationNumber = createIdentificationNumber()
	}
	entryDetail.IndividualName = receiver.Metadata
	entryDetail.TraceNumber = createTraceNumber(origDep.RoutingNumber)

	// TEL transfers use DiscretionaryData for PaymentTypeCode
	if transfer.TELDetail.PaymentType == TELSingle {
		entryDetail.DiscretionaryData = "S"
	} else {
		entryDetail.DiscretionaryData = "R"
		return nil, fmt.Errorf("createTELBatch: %s TEL transfers are not supported yet", TELReoccurring)
	}

	// For now just create PPD
	batch, err := ach.NewBatch(batchHeader)
	if err != nil {
		return nil, fmt.Errorf("ACH file %s (userId=%s): failed to create batch: %v", id, userId, err)
	}
	batch.AddEntry(entryDetail)
	batch.SetControl(ach.NewBatchControl())

	if err := batch.Create(); err != nil {
		return batch, err
	}
	return batch, nil
}

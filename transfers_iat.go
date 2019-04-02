// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
)

type IATDetail struct {
	// Originator information
	OriginatorName        string `json:"originatorName"`
	OriginatorAddress     string `json:"originatorAddress"`
	OriginatorCity        string `json:"originatorCity"`
	OriginatorState       string `json:"originatorState"`
	OriginatorPostalCode  string `json:"originatorPostalCode"`
	OriginatorCountryCode string `json:"originatorCountryCode"`

	// ODFI information
	ODFIName               string `json:"ODFIName"`
	ODFIIDNumberQualifier  string `json:"ODFIIDNumberQualifier"`  // 01 = National Clearing System, 02 = BIC Code, 03 = IBAN Code
	ODFIIdentification     string `json:"ODFIIdentification"`     // TODO(adam): not on example we got, why?
	ODFIBranchCurrencyCode string `json:"ODFIBranchCurrencyCode"` // two-letter ISO code

	// Receiver information
	ReceiverName        string `json:"receiverName"`
	ReceiverAddress     string `json:"receiverAddress"`
	ReceiverCity        string `json:"receiverCity"`
	ReceiverState       string `json:"receiverState"`
	ReceiverPostalCode  string `json:"receiverPostalCode"`
	ReceiverCountryCode string `json:"receiverCountryCode"`

	// RDFI information
	RDFIName               string `json:"RDFIName"`
	RDFIIDNumberQualifier  string `json:"RDFIIDNumberQualifier"`
	RDFIIdentification     string `json:"RDFIIdentification"` // TODO(adam): not on example we got, why?
	RDFIBranchCurrencyCode string `json:"RDFIBranchCurrencyCode"`

	// Foreign Correspondent Bank information
	ForeignCorrespondentBankName              string `json:"foreignCorrespondentBankName"`
	ForeignCorrespondentBankIDNumberQualifier string `json:"foreignCorrespondentBankIDNumberQualifier"` // 01 = National Clearing System, “02” = BIC Code, “03” = IBAN Code
	ForeignCorrespondentBankIDNumber          string `json:"foreignCorrespondentBankIDNumber"`
	ForeignCorrespondentBankBranchCountryCode string `json:"foreignCorrespondentBankBranchCountryCode"` // two-letter ISO code

}

func (iat *IATDetail) validate() error {
	// Our ACH service validates the various ISO codes sent along, so for no we aren't going to double validate.
	// This data isn't stored anywhere, so we aren't at risk of data corruption.
	if iat.OriginatorName == "" || iat.OriginatorAddress == "" || iat.OriginatorCity == "" ||
		iat.OriginatorState == "" || iat.OriginatorPostalCode == "" || iat.OriginatorCountryCode == "" {
		return errors.New("IAT: missing Originator details")
	}
	if iat.ODFIName == "" || iat.ODFIIDNumberQualifier == "" || iat.ODFIIdentification == "" || iat.ODFIBranchCurrencyCode == "" {
		return errors.New("IAT: missing ODFI details")
	}
	if iat.ReceiverName == "" || iat.ReceiverAddress == "" || iat.ReceiverCity == "" || iat.ReceiverState == "" ||
		iat.ReceiverPostalCode == "" || iat.ReceiverCountryCode == "" {
		return errors.New("IAT: missing Receiver details")
	}
	if iat.RDFIName == "" || iat.RDFIIDNumberQualifier == "" || iat.RDFIIdentification == "" || iat.RDFIBranchCurrencyCode == "" {
		return errors.New("IAT: missing RDFI details")
	}
	if iat.ForeignCorrespondentBankName == "" || iat.ForeignCorrespondentBankIDNumberQualifier == "" || iat.ForeignCorrespondentBankIDNumber == "" ||
		iat.ForeignCorrespondentBankBranchCountryCode == "" {
		return errors.New("IAT: missing ForeignCorrespondentBank details")
	}
	return nil
}

func createIATBatch(id, userId string, transfer *Transfer, cust *Customer, custDep *Depository, orig *Originator, origDep *Depository) (*ach.IATBatch, error) {
	if transfer == nil {
		return nil, errors.New("IAT: nil Transfer")
	}
	if err := transfer.IATDetail.validate(); err != nil {
		return nil, err
	}

	batchHeader := ach.NewIATBatchHeader()
	batchHeader.ID = id
	batchHeader.ServiceClassCode = 220

	batchHeader.ForeignExchangeIndicator = "FV"       // Fixed-to-Fixed, could be FV or VF (V=variable)
	batchHeader.ForeignExchangeReferenceIndicator = 1 // Populated by Gateway operator

	// It seems most FI's normally overrides the currency conversation fields with a rate that changes daily.
	batchHeader.ForeignExchangeReference = "1.0" // currency exchange rate

	batchHeader.ISODestinationCountryCode = transfer.IATDetail.ReceiverCountryCode
	batchHeader.ISOOriginatingCurrencyCode = transfer.IATDetail.ODFIBranchCurrencyCode
	batchHeader.ISODestinationCurrencyCode = transfer.IATDetail.RDFIBranchCurrencyCode

	batchHeader.OriginatorIdentification = orig.Identification
	batchHeader.StandardEntryClassCode = strings.ToUpper(transfer.StandardEntryClassCode)
	batchHeader.CompanyEntryDescription = transfer.Description

	batchHeader.EffectiveEntryDate = base.Now().AddBankingDay(1).Format("060102") // Date to be posted, YYMMDD
	batchHeader.OriginatorStatusCode = 0                                          // 0=ACH Operator, 1=Depository FI
	batchHeader.ODFIIdentification = aba8(origDep.RoutingNumber)

	// IAT Entry Detail record
	entryDetail := ach.NewIATEntryDetail()
	entryDetail.ID = id
	entryDetail.TransactionCode = 22
	entryDetail.RDFIIdentification = aba8(custDep.RoutingNumber)
	entryDetail.CheckDigit = abaCheckDigit(custDep.RoutingNumber)
	entryDetail.Amount = transfer.Amount.Int()
	entryDetail.DFIAccountNumber = custDep.AccountNumber
	entryDetail.AddendaRecordIndicator = 1
	entryDetail.TraceNumber = createTraceNumber()
	entryDetail.Category = "Forward"
	entryDetail.SecondaryOFACScreeningIndicator = "1" // Set because we (paygate) checks the OFAC list

	entryDetail.Addenda10 = ach.NewAddenda10()
	// TODO(adam): How do we determine the code value here?
	// ANN = Annuity, BUS = Business/Commercial, DEP = Deposit, LOA = Loan, MIS = Miscellaneous, MOR = Mortgage
	// PEN = Pension, RLS = Rent/Lease, REM = Remittance2, SAL = Salary/Payroll, TAX = Tax, TEL = Telephone-Initiated Transaction
	// WEB = Internet-Initiated Transaction, ARC = Accounts Receivable Entry, BOC = Back Office Conversion Entry,
	// POP = Point of Purchase Entry, RCK = Re-presented Check Entry
	entryDetail.Addenda10.TransactionTypeCode = "WEB"
	entryDetail.Addenda10.ForeignPaymentAmount = transfer.Amount.Int()
	entryDetail.Addenda10.ForeignTraceNumber = entryDetail.TraceNumber
	entryDetail.Addenda10.Name = cust.Metadata

	entryDetail.Addenda11 = ach.NewAddenda11()
	entryDetail.Addenda11.OriginatorName = transfer.IATDetail.OriginatorName
	entryDetail.Addenda11.OriginatorStreetAddress = transfer.IATDetail.OriginatorAddress

	entryDetail.Addenda12 = ach.NewAddenda12()
	entryDetail.Addenda12.OriginatorCityStateProvince = fmt.Sprintf(`%s*%s\`, transfer.IATDetail.OriginatorCity, transfer.IATDetail.OriginatorState)
	entryDetail.Addenda12.OriginatorCountryPostalCode = fmt.Sprintf(`%s*%s\`, transfer.IATDetail.OriginatorCountryCode, transfer.IATDetail.OriginatorPostalCode)

	entryDetail.Addenda13 = ach.NewAddenda13()
	entryDetail.Addenda13.ODFIName = transfer.IATDetail.ODFIName
	entryDetail.Addenda13.ODFIIDNumberQualifier = transfer.IATDetail.ODFIIDNumberQualifier
	entryDetail.Addenda13.ODFIIdentification = transfer.IATDetail.ODFIIdentification
	entryDetail.Addenda13.ODFIBranchCountryCode = transfer.IATDetail.ODFIBranchCurrencyCode

	entryDetail.Addenda14 = ach.NewAddenda14()
	entryDetail.Addenda14.RDFIName = transfer.IATDetail.RDFIName
	entryDetail.Addenda14.RDFIIDNumberQualifier = transfer.IATDetail.RDFIIDNumberQualifier
	entryDetail.Addenda14.RDFIIdentification = transfer.IATDetail.RDFIIdentification
	entryDetail.Addenda14.RDFIBranchCountryCode = transfer.IATDetail.RDFIBranchCurrencyCode

	entryDetail.Addenda15 = ach.NewAddenda15()
	entryDetail.Addenda15.ReceiverIDNumber = transfer.IATDetail.RDFIIDNumberQualifier
	entryDetail.Addenda15.ReceiverStreetAddress = transfer.IATDetail.ReceiverAddress

	entryDetail.Addenda16 = ach.NewAddenda16()
	entryDetail.Addenda16.ReceiverCityStateProvince = fmt.Sprintf(`%s*%s\`, transfer.IATDetail.ReceiverCity, transfer.IATDetail.ReceiverState)
	entryDetail.Addenda16.ReceiverCountryPostalCode = fmt.Sprintf(`%s*%s\`, transfer.IATDetail.ReceiverCountryCode, transfer.IATDetail.OriginatorPostalCode)

	// TODO(adam): Addenda17 is optional, so do we need to include it? I'm unsure how we can get terminal level
	// information without just requiring it in the HTTP API.
	//
	// addenda17 := ach.NewAddenda17()
	// // Terminal ID code (6 chars) * terminal location (27 chars) * terminal city (15 chars) * state / country (2 chars) \
	// // 200509 * 321 east market st * any town * VA\  (no spaces between segments)
	// addenda17.PaymentRelatedInformation = "" // TODO(adam)
	// entryDetail.AddAddenda17(addenda17)

	addenda18 := ach.NewAddenda18()
	addenda18.ForeignCorrespondentBankName = transfer.IATDetail.ForeignCorrespondentBankName
	addenda18.ForeignCorrespondentBankIDNumberQualifier = transfer.IATDetail.ForeignCorrespondentBankIDNumberQualifier
	addenda18.ForeignCorrespondentBankIDNumber = transfer.IATDetail.ForeignCorrespondentBankIDNumber
	addenda18.ForeignCorrespondentBankBranchCountryCode = transfer.IATDetail.ForeignCorrespondentBankBranchCountryCode
	entryDetail.AddAddenda18(addenda18)

	entryDetail.AddendaRecords = 8

	batch := ach.NewIATBatch(batchHeader)
	batch.AddEntry(entryDetail)
	batch.SetHeader(batchHeader)
	batch.SetControl(ach.NewBatchControl())

	if err := batch.Create(); err != nil {
		return &batch, err
	}
	return &batch, nil
}

func createTraceNumber() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1e12))
	return fmt.Sprintf("%d", n.Int64())
}

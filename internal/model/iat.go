// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"errors"
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
	ODFIIDNumberQualifier  string `json:"ODFIIDNumberQualifier"` // 01 = National Clearing System, 02 = BIC Code, 03 = IBAN Code
	ODFIIdentification     string `json:"ODFIIdentification"`
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
	RDFIIdentification     string `json:"RDFIIdentification"`
	RDFIBranchCurrencyCode string `json:"RDFIBranchCurrencyCode"`

	// Foreign Correspondent Bank information
	ForeignCorrespondentBankName              string `json:"foreignCorrespondentBankName"`
	ForeignCorrespondentBankIDNumberQualifier string `json:"foreignCorrespondentBankIDNumberQualifier"` // 01 = National Clearing System, “02” = BIC Code, “03” = IBAN Code
	ForeignCorrespondentBankIDNumber          string `json:"foreignCorrespondentBankIDNumber"`
	ForeignCorrespondentBankBranchCountryCode string `json:"foreignCorrespondentBankBranchCountryCode"` // two-letter ISO code

}

func (iat *IATDetail) Validate() error {
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

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"testing"

	"github.com/moov-io/base"
	customers "github.com/moov-io/customers/pkg/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

func TestPPD__entry(t *testing.T) {
	opts := Options{
		ODFIRoutingNumber: "987654320",
		FileConfig: config.FileConfig{
			Addendum: config.Addendum{
				Create05: true,
			},
		},
	}
	xfer := &client.Transfer{
		Description: "PAYROLL",
		Amount: client.Amount{
			Currency: "USD",
			Value:    10000,
		},
	}
	src := Source{
		Account:       customers.Account{RoutingNumber: "987654320"},
		AccountNumber: "98765",
	}
	dst := Destination{
		Account:       customers.Account{RoutingNumber: "123456780"},
		AccountNumber: "12345",
	}

	ed := createPPDEntry(base.ID(), opts, xfer, src, dst)
	if ed == nil {
		t.Fatal("nil PPD EntryDetail")
	}

	if ed.RDFIIdentification != "12345678" {
		t.Errorf("ed.RDFIIdentification=%s", ed.RDFIIdentification)
	}
	if ed.CheckDigit != "0" {
		t.Errorf("ed.CheckDigit=%s", ed.CheckDigit)
	}
	if ed.DFIAccountNumber != "12345" {
		t.Errorf("ed.DFIAccountNumber=%s", ed.DFIAccountNumber)
	}
	if ed.Amount != 10000 {
		t.Errorf("ed.Amount=%d", ed.Amount)
	}
	if ed.DiscretionaryData != "PAYROLL" {
		t.Errorf("ed.DiscretionaryData=%s", ed.DiscretionaryData)
	}
	if ed.Addenda05[0].PaymentRelatedInformation != "PAYROLL" {
		t.Errorf("ed.Addenda05[0].PaymentRelatedInformation: %q", ed.Addenda05[0].PaymentRelatedInformation)
	}
}

func TestPPD__offset(t *testing.T) {
	opts := Options{
		ODFIRoutingNumber: "987654320",
		FileConfig: config.FileConfig{
			BalanceEntries: true,
			Addendum: config.Addendum{
				Create05: true,
			},
		},
	}
	xfer := &client.Transfer{
		Description: "PAYROLL",
		Amount: client.Amount{
			Currency: "USD",
			Value:    10000,
		},
	}
	src := Source{
		Account:       customers.Account{RoutingNumber: "987654320"},
		AccountNumber: "98765",
	}
	dst := Destination{
		Account:       customers.Account{RoutingNumber: "123456780"},
		AccountNumber: "12345",
	}

	ed := createPPDEntry(base.ID(), opts, xfer, src, dst)
	if ed == nil {
		t.Fatal("nil PPD EntryDetail")
	}
	offset, err := balancePPDEntry(ed, opts, src, dst)
	if ed == nil {
		t.Fatal(err)
	}

	if offset.RDFIIdentification != "98765432" {
		t.Errorf("offset.RDFIIdentification=%s", offset.RDFIIdentification)
	}
	if offset.CheckDigit != "0" {
		t.Errorf("offset.CheckDigit=%s", offset.CheckDigit)
	}
	if offset.DFIAccountNumber != "98765" {
		t.Errorf("offset.DFIAccountNumber=%s", offset.DFIAccountNumber)
	}
	if offset.Amount != 10000 {
		t.Errorf("offset.Amount=%d", offset.Amount)
	}
	if offset.DiscretionaryData != "OFFSET" {
		t.Errorf("offset.DiscretionaryData=%s", offset.DiscretionaryData)
	}
	if offset.Addenda05[0].PaymentRelatedInformation != "OFFSET" {
		t.Errorf("offset.Addenda05[0].PaymentRelatedInformation: %q", offset.Addenda05[0].PaymentRelatedInformation)
	}
}

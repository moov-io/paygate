// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	customers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

func TestFiles__ConstructFile(t *testing.T) {
	transferID := base.ID()
	opts := Options{
		ODFIRoutingNumber: "123456780",
		Gateway: config.Gateway{
			OriginName:      "My Bank",
			DestinationName: "Their Bank",
		},
		FileConfig: config.Transfers{
			BalanceEntries: true,
		},
	}
	companyID := "MOOVZZZZZZ"
	xfer := &client.Transfer{
		Amount:      "USD 12.47",
		Description: "test payment",
	}
	source := Source{
		Customer: customers.Customer{
			FirstName: "John",
			LastName:  "Doe",
		},
		Account: customers.Account{
			RoutingNumber: opts.ODFIRoutingNumber,
			Type:          customers.CHECKING,
		},
		AccountNumber: "7654321",
	}
	destination := Destination{
		Customer: customers.Customer{
			FirstName: "Jane",
			LastName:  "Doe",
		},
		Account: customers.Account{
			RoutingNumber: "987654320",
			Type:          customers.SAVINGS,
		},
		AccountNumber: "1234567",
	}

	file, err := ConstructFile(transferID, opts, companyID, xfer, source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Fatal("expected ach.File to be created")
	}
	if err := file.Validate(); err != nil {
		t.Error(err)
	}

	// sanity check
	if len(file.Batches) != 1 {
		t.Error("unexpected batches")
		for i := range file.Batches {
			t.Errorf("batch #%d: %#v", i, file.Batches[i])
		}
		t.Fatal("")
	}
	entries := file.Batches[0].GetEntries()
	if len(entries) != 2 {
		t.Error("unexpected entries")
		for i := range entries {
			t.Errorf("entry #%d: %#v", i, entries[i])
		}
		t.Fatal("")
	}

	for i := range entries {
		if entries[i].TransactionCode == ach.CheckingCredit {
			if entries[i].RDFIIdentification != "98765432" {
				t.Errorf("RDFIIdentification=%s", entries[i].RDFIIdentification)
			}
			if entries[i].DFIAccountNumber != "1234567" {
				t.Errorf("DFIAccountNumber=%s", entries[i].DFIAccountNumber)
			}
			if entries[i].Amount != 1247 {
				t.Errorf("Amount=%d", entries[i].Amount)
			}
			if entries[i].IndividualName != "Jane Doe" {
				t.Errorf("IndividualName=%q", entries[i].IndividualName)
			}
			continue
		}
		if entries[i].TransactionCode == ach.SavingsDebit {
			if entries[i].RDFIIdentification != "12345678" {
				t.Errorf("RDFIIdentification=%s", entries[i].RDFIIdentification)
			}
			if entries[i].DFIAccountNumber != "7654321" {
				t.Errorf("DFIAccountNumber=%s", entries[i].DFIAccountNumber)
			}
			if entries[i].Amount != 1247 {
				t.Errorf("Amount=%d", entries[i].Amount)
			}
			if entries[i].IndividualName != "John Doe" {
				t.Errorf("IndividualName=%q", entries[i].IndividualName)
			}
			continue
		}
		t.Errorf("unexpected entry: %#v", entries[i])
	}
}

func TestFiles__determineOrigin(t *testing.T) {
	opts := Options{
		ODFIRoutingNumber: "987654320",
	}
	if v := determineOrigin(opts); v != "987654320" {
		t.Errorf("origin=%q", v)
	}

	opts.Gateway.Origin = "Moov"
	if v := determineOrigin(opts); v != "Moov" {
		t.Errorf("origin=%q", v)
	}
}

func TestFiles__determineDestination(t *testing.T) {
	opts := Options{
		ODFIRoutingNumber: "987654320",
	}
	src := Source{
		Account: customers.Account{
			RoutingNumber: "987654320",
		},
	}
	dest := Destination{
		Account: customers.Account{
			RoutingNumber: "123456780",
		},
	}

	if v := determineDestination(opts, src, dest); v != "123456780" {
		t.Errorf("destination=%q", v)
	}

	src.Account.RoutingNumber = "123456780"
	dest.Account.RoutingNumber = "987654320"
	if v := determineDestination(opts, src, dest); v != "123456780" {
		t.Errorf("destination=%q", v)
	}

	opts.Gateway.Destination = "Moov"
	if v := determineDestination(opts, src, dest); v != "Moov" {
		t.Errorf("destination=%q", v)
	}
}

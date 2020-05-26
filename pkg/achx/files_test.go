// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"testing"

	"github.com/moov-io/base"
	customers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

func TestFiles__ConstructFile(t *testing.T) {
	transferID := base.ID()
	odfi := config.ODFI{
		RoutingNumber: "323274270",
		Gateway: config.Gateway{
			OriginName:      "My Bank",
			DestinationName: "Their Bank",
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
			RoutingNumber: odfi.RoutingNumber,
			Type:          customers.CHECKING,
		},
	}
	destination := Destination{
		Customer: customers.Customer{},
		Account: customers.Account{
			RoutingNumber: "273976369",
			Type:          customers.SAVINGS,
		},
		AccountNumber: "1234567",
	}

	file, err := ConstrctFile(transferID, odfi, companyID, xfer, source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Fatal("expected ach.File to be created")
	}
	if err := file.Validate(); err != nil {
		t.Error(err)
	}
}

func TestFiles__determineOrigin(t *testing.T) {
	odfi := config.ODFI{
		RoutingNumber: "987654320",
	}
	if v := determineOrigin(odfi); v != "987654320" {
		t.Errorf("origin=%q", v)
	}

	odfi.Gateway.Origin = "Moov"
	if v := determineOrigin(odfi); v != "Moov" {
		t.Errorf("origin=%q", v)
	}
}

func TestFiles__determineDestination(t *testing.T) {
	odfi := config.ODFI{RoutingNumber: "987654320"}
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

	if v := determineDestination(odfi, src, dest); v != "123456780" {
		t.Errorf("destination=%q", v)
	}

	src.Account.RoutingNumber = "123456780"
	dest.Account.RoutingNumber = "987654320"
	if v := determineDestination(odfi, src, dest); v != "123456780" {
		t.Errorf("destination=%q", v)
	}

	odfi.Gateway.Destination = "Moov"
	if v := determineDestination(odfi, src, dest); v != "Moov" {
		t.Errorf("destination=%q", v)
	}
}

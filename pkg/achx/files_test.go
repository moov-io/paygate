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
			RoutingNumber: "323274270",
			Type:          customers.CHECKING,
		},
	}
	destination := Destination{
		Customer: customers.Customer{},
		Account: customers.Account{
			RoutingNumber: "273976369",
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

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fundflow

import (
	"strings"
	"testing"

	customers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

func TestOriginate__DebitCheck(t *testing.T) {
	cfg := config.Empty()
	cfg.ODFI.RoutingNumber = "987654320"

	fp := NewFirstPerson(cfg.Logger, cfg.ODFI)

	companyID := "MOOV"
	xfer := &client.Transfer{}
	src := Source{
		Customer: customers.Customer{
			Status: customers.RECEIVE_ONLY,
		},
	}
	dest := Destination{
		Account: customers.Account{
			RoutingNumber: "987654320",
		},
	}
	if _, err := fp.Originate(companyID, xfer, src, dest); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "does not support debit") {
			t.Error(err)
		}
	}
}

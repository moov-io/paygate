// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"strings"
	"testing"
	"time"

	customers "github.com/moov-io/customers/pkg/client"
	"github.com/moov-io/paygate/pkg/client"
)

func TestBatch__SameDay(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{
		ODFIRoutingNumber:     "987654320",
		CutoffTimezone:        loc,
		CompanyIdentification: "Moov",
	}
	xfer := &client.Transfer{
		Description: "PAYROLL",
		SameDay:     true,
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
	}
	bh := makeBatchHeader("", opts, xfer, source)
	if bh == nil {
		t.Fatal("nil BatchHeader")
	}

	if !strings.HasPrefix(bh.CompanyDescriptiveDate, "SD") {
		t.Errorf("CompanyDescriptiveDate=%q", bh.CompanyDescriptiveDate)
	}
}

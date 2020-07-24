// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"strings"
	"testing"

	"github.com/moov-io/base"
	moovcustomers "github.com/moov-io/customers/client"
)

func TestHealthChecker(t *testing.T) {
	client := &MockClient{
		Accounts: make(map[string]*moovcustomers.Account),
	}
	customerID, accountID := base.ID(), base.ID()

	if err := HealthChecker(client, customerID, accountID)(); err != nil {
		if !strings.Contains(err.Error(), "unable to find customerID") {
			t.Fatal(err)
		}
	}

	// find a customer, but no account
	client.Customer = &moovcustomers.Customer{
		CustomerID: customerID,
		Status:     moovcustomers.VERIFIED,
	}
	if err := HealthChecker(client, customerID, accountID)(); err != nil {
		if !strings.Contains(err.Error(), "unable to find accountID") {
			t.Fatal(err)
		}
	}

	// find an account
	client.Accounts[accountID] = &moovcustomers.Account{
		AccountID: accountID,
		Status:    moovcustomers.VALIDATED,
	}
	if err := HealthChecker(client, customerID, accountID)(); err != nil {
		t.Fatal(err)
	}
}

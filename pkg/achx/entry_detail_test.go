// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"testing"

	"github.com/moov-io/ach"
	customers "github.com/moov-io/customers/client"
)

func TestEntryDetail_TransactionCodeCredit(t *testing.T) {
	opts := Options{}
	destinationAccount := customers.Account{}

	if n := determineTransactionCode(opts, destinationAccount); n != 0 {
		t.Errorf("unexpected TransactionCode=%d", n)
	}

	opts.ODFIRoutingNumber = "987654320"
	destinationAccount.RoutingNumber = "987654320"
	destinationAccount.Type = customers.CHECKING
	if n := determineTransactionCode(opts, destinationAccount); n != ach.CheckingCredit {
		t.Errorf("unexpected TransactionCode=%d", n)
	}

	destinationAccount.Type = customers.SAVINGS
	if n := determineTransactionCode(opts, destinationAccount); n != ach.SavingsCredit {
		t.Errorf("unexpected TransactionCode=%d", n)
	}
}

func TestEntryDetail_TransactionCodeDebit(t *testing.T) {
	opts := Options{}
	destinationAccount := customers.Account{}

	if n := determineTransactionCode(opts, destinationAccount); n != 0 {
		t.Errorf("unexpected TransactionCode=%d", n)
	}

	opts.ODFIRoutingNumber = "987654320"
	destinationAccount.RoutingNumber = "123456780"
	destinationAccount.Type = customers.CHECKING
	if n := determineTransactionCode(opts, destinationAccount); n != ach.CheckingDebit {
		t.Errorf("unexpected TransactionCode=%d", n)
	}

	destinationAccount.Type = customers.SAVINGS
	if n := determineTransactionCode(opts, destinationAccount); n != ach.SavingsDebit {
		t.Errorf("unexpected TransactionCode=%d", n)
	}
}

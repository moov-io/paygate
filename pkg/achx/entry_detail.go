// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"github.com/moov-io/ach"
	customers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/config"
)

func determineTransactionCode(odfi config.ODFI, srcAcct customers.Account) int {
	if odfi.RoutingNumber == srcAcct.RoutingNumber {
		// Credit
		if srcAcct.Type == customers.CHECKING {
			return ach.CheckingCredit
		}
		if srcAcct.Type == customers.SAVINGS {
			return ach.SavingsCredit
		}
	}
	// Debit
	if srcAcct.Type == customers.CHECKING {
		return ach.CheckingDebit
	}
	if srcAcct.Type == customers.SAVINGS {
		return ach.SavingsDebit
	}
	return 0 // invalid, represents a logic bug
}

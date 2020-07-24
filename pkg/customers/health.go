// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"fmt"
)

type HealthCheck func() error

// HealthChecker verifies the provided customerID and accountID exist and are in acceptable statuses
// to be used in Transfers. This is used to verify configured Sources such as micro-deposits.
func HealthChecker(client Client, customerID, accountID string) HealthCheck {
	// Check the Customer
	cust, err := client.Lookup(customerID, "health-check", "paygate")
	if err != nil {
		return failure(fmt.Errorf("customerID=%s failure: %v", customerID, err))
	}
	if cust == nil || cust.CustomerID == "" {
		return failure(fmt.Errorf("unable to find customerID=%s", customerID))
	}
	if err := AcceptableCustomerStatus(cust); err != nil {
		return failure(err)
	}

	// Check the Account
	acct, err := client.FindAccount(customerID, accountID)
	if err != nil {
		return failure(fmt.Errorf("accountID=%s failure: %v", accountID, err))
	}
	if acct == nil || acct.AccountID == "" {
		return failure(fmt.Errorf("unable to find accountID=%s", accountID))
	}
	if err := AcceptableAccountStatus(acct); err != nil {
		return failure(err)
	}

	return success()
}

func success() HealthCheck {
	return func() error {
		return nil
	}
}

func failure(err error) HealthCheck {
	return func() error {
		return err
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"fmt"
	"strings"

	moovcustomers "github.com/moov-io/customers/pkg/client"
)

// AcceptableCustomerStatus returns an error if the Customer's status
// can not be used in a Transfer.
func AcceptableCustomerStatus(cust *moovcustomers.Customer) error {
	switch {
	case strings.EqualFold(string(cust.Status), string(moovcustomers.RECEIVE_ONLY)) || strings.EqualFold(string(cust.Status), string(moovcustomers.VERIFIED)):
		return nil // valid status, do nothing
	}
	return fmt.Errorf("customerID=%s has unacceptable status: %s", cust.CustomerID, cust.Status)
}

// AcceptableAccountStatus returns an error if the Accounts's status
// can not be used in a Transfer.
func AcceptableAccountStatus(acct *moovcustomers.Account) error {
	if !strings.EqualFold(string(acct.Status), string(moovcustomers.VALIDATED)) {
		return fmt.Errorf("accountID=%s has unacceptable status: %s", acct.AccountID, acct.Status)
	}
	return nil
}

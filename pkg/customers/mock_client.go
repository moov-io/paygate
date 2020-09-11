// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	moovcustomers "github.com/moov-io/customers/pkg/client"
)

type MockClient struct {
	Accounts  map[string]*moovcustomers.Account
	Customers []*moovcustomers.Customer
	Transit   *moovcustomers.TransitAccountNumber
	Result    *OfacSearch

	Err error
}

func (c *MockClient) Ping() error {
	return c.Err
}

func (c *MockClient) Lookup(customerID string, requestID string, tenantID string) (*moovcustomers.Customer, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	for i := range c.Customers {
		if c.Customers[i].CustomerID == customerID {
			return c.Customers[i], nil
		}
	}
	return nil, nil
}

func (c *MockClient) FindAccount(customerID, accountID string) (*moovcustomers.Account, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	if acct, exists := c.Accounts[accountID]; exists {
		return acct, nil
	}
	return nil, nil
}

func (c *MockClient) DecryptAccount(customerID, accountID string) (*moovcustomers.TransitAccountNumber, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Transit, nil
}

func (c *MockClient) LatestOFACSearch(customerID, requestID string, tenantID string) (*OfacSearch, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Result, nil
}

func (c *MockClient) RefreshOFACSearch(customerID, requestID string, tenantID string) (*OfacSearch, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Result, nil
}

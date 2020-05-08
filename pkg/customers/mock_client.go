// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	moovcustomers "github.com/moov-io/customers/client"
)

type MockClient struct {
	Account  *moovcustomers.Account
	Transit  *moovcustomers.TransitAccountNumber
	Customer *moovcustomers.Customer
	Result   *OfacSearch

	Err error
}

func (c *MockClient) Ping() error {
	return c.Err
}

func (c *MockClient) Lookup(customerID string, requestID string, userID string) (*moovcustomers.Customer, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Customer, nil
}

func (c *MockClient) FindAccount(customerID, accountID string) (*moovcustomers.Account, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Account, nil
}

func (c *MockClient) DecryptAccount(customerID, accountID string) (*moovcustomers.TransitAccountNumber, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Transit, nil
}

func (c *MockClient) LatestOFACSearch(customerID, requestID string, userID string) (*OfacSearch, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Result, nil
}

func (c *MockClient) RefreshOFACSearch(customerID, requestID string, userID string) (*OfacSearch, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Result, nil
}

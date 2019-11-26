// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	moovcustomers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/id"
)

type TestClient struct {
	Customer    *moovcustomers.Customer
	Disclaimers []moovcustomers.Disclaimer
	Result      *moovcustomers.OfacSearch

	Err error
}

func (c *TestClient) Ping() error {
	return c.Err
}

func (c *TestClient) Create(opts *Request) (*moovcustomers.Customer, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Customer, nil
}

func (c *TestClient) Lookup(customerID string, requestID string, userID id.User) (*moovcustomers.Customer, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Customer, nil
}

func (c *TestClient) GetDisclaimers(customerID, requestID string, userID id.User) ([]moovcustomers.Disclaimer, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Disclaimers, nil
}

func (c *TestClient) LatestOFACSearch(customerID, requestID string, userID id.User) (*moovcustomers.OfacSearch, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Result, nil
}

func (c *TestClient) RefreshOFACSearch(customerID, requestID string, userID id.User) (*moovcustomers.OfacSearch, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Result, nil
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"github.com/moov-io/paygate/pkg/id"
)

type TestClient struct {
	Customer    *Customer
	Disclaimers []Disclaimer
	Result      *OfacSearch

	Err error
}

func (c *TestClient) Ping() error {
	return c.Err
}

func (c *TestClient) Create(opts *Request) (*Customer, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Customer, nil
}

func (c *TestClient) Lookup(customerID string, requestID string, userID id.User) (*Customer, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Customer, nil
}

func (c *TestClient) GetDisclaimers(customerID, requestID string, userID id.User) ([]Disclaimer, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Disclaimers, nil
}

func (c *TestClient) LatestOFACSearch(customerID, requestID string, userID id.User) (*OfacSearch, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Result, nil
}

func (c *TestClient) RefreshOFACSearch(customerID, requestID string, userID id.User) (*OfacSearch, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Result, nil
}

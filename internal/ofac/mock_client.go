// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package ofac

import (
	"context"

	moovofac "github.com/moov-io/ofac/client"
)

type TestClient struct {
	Company  *moovofac.OfacCompany
	Customer *moovofac.OfacCustomer
	SDN      *moovofac.Sdn

	// error to be returned instead of field from above
	Err error
}

func (c *TestClient) Ping() error {
	return c.Err
}

func (c *TestClient) GetCompany(_ context.Context, id string) (*moovofac.OfacCompany, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Company, nil
}

func (c *TestClient) GetCustomer(_ context.Context, id string) (*moovofac.OfacCustomer, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.Customer, nil
}

func (c *TestClient) Search(_ context.Context, name string, _ string) (*moovofac.Sdn, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return c.SDN, nil
}

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	moovcustomers "github.com/moov-io/customers/client"
)

type TestClient struct {
	Customer *moovcustomers.Customer

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

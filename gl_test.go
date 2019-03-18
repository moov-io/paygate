// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	gl "github.com/moov-io/gl/client"
)

type testGLClient struct {
	accounts []gl.Account

	err error
}

func (c *testGLClient) Ping() error {
	return c.err
}

func (c *testGLClient) GetAccounts(customerId string) ([]gl.Account, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.accounts, nil
}

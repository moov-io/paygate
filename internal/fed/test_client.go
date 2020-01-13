// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fed

type TestClient struct {
	Err error
}

func (c *TestClient) Ping() error {
	return c.Err
}

func (c *TestClient) LookupRoutingNumber(routingNumber string) error {
	return c.Err
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fundflow

import (
	"github.com/moov-io/ach"
	customers "github.com/moov-io/customers/pkg/client"
	"github.com/moov-io/paygate/pkg/client"
)

type Strategy interface {
	Originate(companyID string, xfer *client.Transfer, source Source, destination Destination) ([]*ach.File, error)
	HandleReturn(returned *ach.File, xfer *client.Transfer) ([]*ach.File, error)
}

type Source struct {
	Customer customers.Customer
	Account  customers.Account

	// AccountNumber contains the decrypted account number from the customers service
	AccountNumber string
}

type Destination struct {
	Customer customers.Customer
	Account  customers.Account

	// AccountNumber contains the decrypted account number from the customers service
	AccountNumber string
}

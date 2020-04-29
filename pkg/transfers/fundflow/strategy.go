// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fundflow

import (
	"github.com/moov-io/ach"
	customers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/client"
)

// okay, so we need to figure out both strategies for moving funds

// first-party:
//  PayGate runs as an ACH originator at an FI. This means transfers interact with
//  an account at the FI.
//
//  Outgoing credits are debited from this FI's account without
//  delay and the credits are posted by the RDFI when the file is received.
//
//  Debiting the remote account means we'll credit our account, but typically hold those
//  funds for a settlement period.
//
// These transfers involve one file with an optional return which reverses the initial transfer.

// third-party:
//  PayGate runs at an ODFI but interacts with two FI's to make transfers. There's always
//  a debit and a credit, but two models for when the credit posts.
//   - risk-tolerance: based on multiple (often proprietary) factors we hold the credit or not
//   - pre-funding: n dollars are added to the account and used as tolerance for transfers.
//                  Any transfer which would allow a positive amount is allowed.
//   - line-of-credit: similar to pre-funding, but we're allowed to float n dollars of transfers
//                     up to the credit line's limit
//
// These transfers require at least two files, one debit which can block the resulting credit
// or are marked as FAILED on a return which can't be retried.

// first-party behavior:
//  - given a transfer, create the file and upload it
//  - given a return, find the transfer and reverse it

// third-party behavior:
//  - given a transfer, debit the account
//    - given a return, mark the transfer as FAILED
//  - according to risk credit the other account
//    - given a return, mark the transfer as FAILED

type Strategy interface {
	Originate(xfer *client.Transfer, source Source, destination Destination) ([]*ach.File, error)
	HandleReturn(returned *ach.File, xfer *client.Transfer) ([]*ach.File, error)
}

type Source struct {
	Customer customers.Customer
	Account  customers.Account
}

type Destination struct {
	Customer customers.Customer
	Account  customers.Account

	// AccountNumber contains the decrypted account number from the customers service
	AccountNumber string
}

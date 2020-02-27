// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package accounts

import (
	"errors"

	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockClient struct {
	Accounts    []Account
	Transaction *Transaction

	PostedTransactions []MockTx

	Err error
}

type MockTx struct {
	Lines []TransactionLine
}

func (c *MockClient) Ping() error {
	return c.Err
}

func (c *MockClient) PostTransaction(requestID string, userID id.User, lines []TransactionLine) (*Transaction, error) {
	if len(lines) == 0 {
		return nil, errors.New("no TransactionLine's")
	}
	if c.Err != nil {
		return nil, c.Err
	}
	c.PostedTransactions = append(c.PostedTransactions, MockTx{
		Lines: lines,
	})
	return c.Transaction, nil // yea, this doesn't match, but callers are expected to override MockClient properties
}

func (c *MockClient) SearchAccounts(requestID string, userID id.User, dep *model.Depository) (*Account, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	if len(c.Accounts) > 0 {
		return &c.Accounts[0], nil
	}
	return nil, nil
}

func (c *MockClient) ReverseTransaction(requestID string, userID id.User, transactionID string) error {
	return c.Err
}

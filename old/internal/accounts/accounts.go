// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package accounts

type Account struct {
	ID      string
	Balance int32

	AccountNumber string
	RoutingNumber string
	Type          string
}

type Transaction struct {
	ID string
}

type TransactionLine struct {
	AccountID string
	Purpose   string
	Amount    int32
}

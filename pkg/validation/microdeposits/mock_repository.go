// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"github.com/moov-io/paygate/pkg/client"
)

type mockRepository struct {
	Micro *client.MicroDeposits
	Err   error
}

func (r *mockRepository) getMicroDeposits(microDepositID string) (*client.MicroDeposits, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Micro, nil
}

func (r *mockRepository) getAccountMicroDeposits(accountID string) (*client.MicroDeposits, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Micro, nil
}

func (r *mockRepository) writeMicroDeposits(micro *client.MicroDeposits) error {
	return r.Err
}

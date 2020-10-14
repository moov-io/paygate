// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"time"

	"github.com/moov-io/paygate/pkg/client"
)

type MockRepository struct {
	Transfers []*client.Transfer
	Err       error
}

func (r *MockRepository) getTransfers(organization string, params transferFilterParams) ([]*client.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Transfers, nil
}

func (r *MockRepository) GetTransfer(id string) (*client.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Transfers) > 0 {
		return r.Transfers[0], nil
	}
	return nil, nil
}

func (r *MockRepository) UpdateTransferStatus(transferID string, status client.TransferStatus) error {
	return r.Err
}

func (r *MockRepository) WriteUserTransfer(organization string, transfer *client.Transfer) error {
	return r.Err
}

func (r *MockRepository) deleteUserTransfer(organization string, transferID string) error {
	return r.Err
}

func (r *MockRepository) SaveReturnCode(transferID string, returnCode string) error {
	return r.Err
}

func (r *MockRepository) saveTraceNumbers(transferID string, traceNumbers []string) error {
	return r.Err
}

func (r *MockRepository) LookupTransferFromReturn(amount client.Amount, traceNumber string, effectiveEntryDate time.Time) (*client.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Transfers) > 0 {
		return r.Transfers[0], nil
	}
	return nil, nil
}

func (r *MockRepository) getTraceNumbers(transferID string) ([]string, error) {
	return []string{
		"123",
		"245",
	}, nil
}

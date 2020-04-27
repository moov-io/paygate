// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/id"
)

type MockRepository struct {
	Transfers []*client.Transfer
	Err       error
}

func (r *MockRepository) GetTransfer(id id.Transfer) (*client.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Transfers) > 0 {
		return r.Transfers[0], nil
	}
	return nil, nil
}

func (r *MockRepository) UpdateTransferStatus(id id.Transfer, status client.TransferStatus) error {
	return r.Err
}

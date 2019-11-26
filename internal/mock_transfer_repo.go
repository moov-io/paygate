// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/id"
)

type MockTransferRepository struct {
	Xfer   *Transfer
	FileID string

	Cur *TransferCursor

	Err error

	// Updated fields
	ReturnCode string
	Status     TransferStatus
}

func (r *MockTransferRepository) getUserTransfers(userID id.User) ([]*Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if r.Xfer != nil {
		return []*Transfer{r.Xfer}, nil
	}
	return nil, nil
}

func (r *MockTransferRepository) getUserTransfer(id TransferID, userID id.User) (*Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Xfer, nil
}

func (r *MockTransferRepository) UpdateTransferStatus(id TransferID, status TransferStatus) error {
	r.Status = status
	return r.Err
}

func (r *MockTransferRepository) GetFileIDForTransfer(id TransferID, userID id.User) (string, error) {
	if r.Err != nil {
		return "", r.Err
	}
	return r.FileID, nil
}

func (r *MockTransferRepository) LookupTransferFromReturn(sec string, amount *Amount, traceNumber string, effectiveEntryDate time.Time) (*Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Xfer, nil
}

func (r *MockTransferRepository) SetReturnCode(id TransferID, returnCode string) error {
	r.ReturnCode = returnCode
	return r.Err
}

func (r *MockTransferRepository) GetTransferCursor(batchSize int, depRepo DepositoryRepository) *TransferCursor {
	return r.Cur
}

func (r *MockTransferRepository) MarkTransferAsMerged(id TransferID, filename string, traceNumber string) error {
	return r.Err
}

func (r *MockTransferRepository) createUserTransfers(userID id.User, requests []*transferRequest) ([]*Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	var transfers []*Transfer
	for i := range requests {
		transfers = append(transfers, requests[i].asTransfer(base.ID()))
	}
	return transfers, nil
}

func (r *MockTransferRepository) deleteUserTransfer(id TransferID, userID id.User) error {
	return r.Err
}

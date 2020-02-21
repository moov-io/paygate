// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockTransferRepository struct {
	Xfer   *model.Transfer
	FileID string

	Cur *TransferCursor

	Err error

	// Updated fields
	ReturnCode string
	Status     model.TransferStatus
}

func (r *MockTransferRepository) getUserTransfers(userID id.User) ([]*model.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if r.Xfer != nil {
		return []*model.Transfer{r.Xfer}, nil
	}
	return nil, nil
}

func (r *MockTransferRepository) getUserTransfer(id id.Transfer, userID id.User) (*model.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Xfer, nil
}

func (r *MockTransferRepository) UpdateTransferStatus(id id.Transfer, status model.TransferStatus) error {
	r.Status = status
	return r.Err
}

func (r *MockTransferRepository) GetFileIDForTransfer(id id.Transfer, userID id.User) (string, error) {
	if r.Err != nil {
		return "", r.Err
	}
	return r.FileID, nil
}

func (r *MockTransferRepository) LookupTransferFromReturn(sec string, amount *model.Amount, traceNumber string, effectiveEntryDate time.Time) (*model.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Xfer, nil
}

func (r *MockTransferRepository) SetReturnCode(id id.Transfer, returnCode string) error {
	r.ReturnCode = returnCode
	return r.Err
}

func (r *MockTransferRepository) GetTransferCursor(batchSize int, depRepo depository.Repository) *TransferCursor {
	return r.Cur
}

func (r *MockTransferRepository) MarkTransferAsMerged(id id.Transfer, filename string, traceNumber string) error {
	return r.Err
}

func (r *MockTransferRepository) createUserTransfers(userID id.User, requests []*transferRequest) ([]*model.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	var transfers []*model.Transfer
	for i := range requests {
		transfers = append(transfers, requests[i].asTransfer(base.ID()))
	}
	return transfers, nil
}

func (r *MockTransferRepository) deleteUserTransfer(id id.Transfer, userID id.User) error {
	return r.Err
}

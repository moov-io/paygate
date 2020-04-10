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

type MockRepository struct {
	Xfer        *model.Transfer
	FileID      string
	TraceNumber string

	Cur *Cursor

	Err error

	// Updated fields
	ReturnCode string
	Status     model.TransferStatus
}

func (r *MockRepository) getUserTransfers(userID id.User, params transferFilterParams) ([]*model.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if r.Xfer != nil {
		return []*model.Transfer{r.Xfer}, nil
	}
	return nil, nil
}

func (r *MockRepository) GetTransfer(id id.Transfer) (*model.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Xfer, nil
}

func (r *MockRepository) getUserTransfer(id id.Transfer, userID id.User) (*model.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Xfer, nil
}

func (r *MockRepository) UpdateTransferStatus(id id.Transfer, status model.TransferStatus) error {
	r.Status = status
	return r.Err
}

func (r *MockRepository) GetFileIDForTransfer(id id.Transfer, userID id.User) (string, error) {
	if r.Err != nil {
		return "", r.Err
	}
	return r.FileID, nil
}

func (r *MockRepository) GetTraceNumber(id id.Transfer) (string, error) {
	if r.Err != nil {
		return "", r.Err
	}
	return r.TraceNumber, nil
}

func (r *MockRepository) LookupTransferFromReturn(sec string, amount *model.Amount, traceNumber string, effectiveEntryDate time.Time) (*model.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Xfer, nil
}

func (r *MockRepository) SetReturnCode(id id.Transfer, returnCode string) error {
	r.ReturnCode = returnCode
	return r.Err
}

func (r *MockRepository) GetCursor(batchSize int, depRepo depository.Repository) *Cursor {
	return r.Cur
}

func (r *MockRepository) MarkTransferAsMerged(id id.Transfer, filename string, traceNumber string) error {
	return r.Err
}

func (r *MockRepository) createUserTransfers(userID id.User, requests []*transferRequest) ([]*model.Transfer, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	var transfers []*model.Transfer
	for i := range requests {
		transfers = append(transfers, requests[i].asTransfer(base.ID()))
	}
	return transfers, nil
}

func (r *MockRepository) deleteUserTransfer(id id.Transfer, userID id.User) error {
	return r.Err
}

func (r *MockRepository) MarkTransfersAsProcessed(filename string, traceNumbers []string) (int64, error) {
	if r.Err != nil {
		return 0, r.Err
	}
	return 0, nil
}

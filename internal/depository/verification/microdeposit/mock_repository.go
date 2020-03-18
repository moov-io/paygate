// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockRepository struct {
	MicroDeposits []*MicroDeposit
	Err           error

	Cur *Cursor
}

func (r *MockRepository) getMicroDeposits(id id.Depository) ([]*MicroDeposit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.MicroDeposits, nil
}

func (r *MockRepository) getMicroDepositsForUser(id id.Depository, userID id.User) ([]*MicroDeposit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.MicroDeposits, nil
}

func (r *MockRepository) InitiateMicroDeposits(id id.Depository, userID id.User, microDeposit []*MicroDeposit) error {
	return r.Err
}

func (r *MockRepository) confirmMicroDeposits(id id.Depository, userID id.User, amounts []model.Amount) error {
	return r.Err
}

func (r *MockRepository) LookupMicroDepositFromReturn(id id.Depository, amount *model.Amount) (*MicroDeposit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.MicroDeposits) > 0 {
		return r.MicroDeposits[0], nil
	}
	return nil, nil
}

func (r *MockRepository) MarkMicroDepositAsMerged(filename string, mc UploadableMicroDeposit) error {
	return r.Err
}

func (r *MockRepository) GetCursor(batchSize int) *Cursor {
	return r.Cur
}

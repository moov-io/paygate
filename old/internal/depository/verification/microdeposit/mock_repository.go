// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockRepository struct {
	Credits    []*Credit
	ReturnCode string
	Err        error

	Cur *Cursor
}

func (r *MockRepository) getMicroDeposits(id id.Depository) ([]*Credit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Credits, nil
}

func (r *MockRepository) GetMicroDepositsForUser(id id.Depository, userID id.User) ([]*Credit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Credits, nil
}

func (r *MockRepository) InitiateMicroDeposits(id id.Depository, userID id.User, microDeposit []*Credit) error {
	return r.Err
}

func (r *MockRepository) confirmMicroDeposits(id id.Depository, userID id.User, amounts []model.Amount) error {
	return r.Err
}

func (r *MockRepository) LookupMicroDepositFromReturn(id id.Depository, amount *model.Amount) (*Credit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Credits) > 0 {
		return r.Credits[0], nil
	}
	return nil, nil
}

func (r *MockRepository) SetReturnCode(id id.Depository, amount model.Amount, returnCode string) error {
	r.ReturnCode = returnCode
	return r.Err
}

func (r *MockRepository) MarkMicroDepositAsMerged(filename string, mc UploadableCredit) error {
	return r.Err
}

func (r *MockRepository) GetCursor(batchSize int) *Cursor {
	return r.Cur
}

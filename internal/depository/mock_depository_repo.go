// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockRepository struct {
	Depositories  []*model.Depository
	MicroDeposits []*MicroDeposit
	Err           error

	DepID string

	Cur *MicroDepositCursor

	// Updated fields
	Status     model.DepositoryStatus
	ReturnCode string
}

func (r *MockRepository) GetDepository(id id.Depository) (*model.Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Depositories) > 0 {
		return r.Depositories[0], nil
	}
	return nil, nil
}

func (r *MockRepository) GetUserDepositories(userID id.User) ([]*model.Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Depositories, nil
}

func (r *MockRepository) GetUserDepository(id id.Depository, userID id.User) (*model.Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Depositories) > 0 {
		return r.Depositories[0], nil
	}
	return nil, nil
}

func (r *MockRepository) UpsertUserDepository(userID id.User, dep *model.Depository) error {
	return r.Err
}

func (r *MockRepository) UpdateDepositoryStatus(id id.Depository, status model.DepositoryStatus) error {
	r.Status = status
	return r.Err
}

func (r *MockRepository) deleteUserDepository(id id.Depository, userID id.User) error {
	return r.Err
}

func (r *MockRepository) GetMicroDeposits(id id.Depository) ([]*MicroDeposit, error) {
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

func (r *MockRepository) LookupDepositoryFromReturn(routingNumber string, accountNumber string) (*model.Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Depositories) > 0 {
		return r.Depositories[0], nil
	}
	return nil, nil
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

func (r *MockRepository) SetReturnCode(id id.Depository, amount model.Amount, returnCode string) error {
	r.ReturnCode = returnCode
	return r.Err
}

func (r *MockRepository) InitiateMicroDeposits(id id.Depository, userID id.User, microDeposit []*MicroDeposit) error {
	return r.Err
}

func (r *MockRepository) confirmMicroDeposits(id id.Depository, userID id.User, amounts []model.Amount) error {
	return r.Err
}

func (r *MockRepository) GetMicroDepositCursor(batchSize int) *MicroDepositCursor {
	return r.Cur
}

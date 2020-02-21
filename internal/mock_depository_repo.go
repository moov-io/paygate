// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockDepositoryRepository struct {
	Depositories  []*Depository
	MicroDeposits []*MicroDeposit
	Err           error

	DepID string

	Cur *MicroDepositCursor

	// Updated fields
	Status     DepositoryStatus
	ReturnCode string
}

func (r *MockDepositoryRepository) GetDepository(id id.Depository) (*Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Depositories) > 0 {
		return r.Depositories[0], nil
	}
	return nil, nil
}

func (r *MockDepositoryRepository) GetUserDepositories(userID id.User) ([]*Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Depositories, nil
}

func (r *MockDepositoryRepository) GetUserDepository(id id.Depository, userID id.User) (*Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Depositories) > 0 {
		return r.Depositories[0], nil
	}
	return nil, nil
}

func (r *MockDepositoryRepository) UpsertUserDepository(userID id.User, dep *Depository) error {
	return r.Err
}

func (r *MockDepositoryRepository) UpdateDepositoryStatus(id id.Depository, status DepositoryStatus) error {
	r.Status = status
	return r.Err
}

func (r *MockDepositoryRepository) deleteUserDepository(id id.Depository, userID id.User) error {
	return r.Err
}

func (r *MockDepositoryRepository) GetMicroDeposits(id id.Depository) ([]*MicroDeposit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.MicroDeposits, nil
}

func (r *MockDepositoryRepository) getMicroDepositsForUser(id id.Depository, userID id.User) ([]*MicroDeposit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.MicroDeposits, nil
}

func (r *MockDepositoryRepository) LookupDepositoryFromReturn(routingNumber string, accountNumber string) (*Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Depositories) > 0 {
		return r.Depositories[0], nil
	}
	return nil, nil
}

func (r *MockDepositoryRepository) LookupMicroDepositFromReturn(id id.Depository, amount *model.Amount) (*MicroDeposit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.MicroDeposits) > 0 {
		return r.MicroDeposits[0], nil
	}
	return nil, nil
}

func (r *MockDepositoryRepository) SetReturnCode(id id.Depository, amount model.Amount, returnCode string) error {
	r.ReturnCode = returnCode
	return r.Err
}

func (r *MockDepositoryRepository) InitiateMicroDeposits(id id.Depository, userID id.User, microDeposit []*MicroDeposit) error {
	return r.Err
}

func (r *MockDepositoryRepository) confirmMicroDeposits(id id.Depository, userID id.User, amounts []model.Amount) error {
	return r.Err
}

func (r *MockDepositoryRepository) GetMicroDepositCursor(batchSize int) *MicroDepositCursor {
	return r.Cur
}

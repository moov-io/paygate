// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockRepository struct {
	DepID        string
	Depositories []*model.Depository
	Status       model.DepositoryStatus

	Err error
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

func (r *MockRepository) getUserDepositories(userID id.User) ([]*model.Depository, error) {
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

func (r *MockRepository) LookupDepositoryFromReturn(routingNumber string, accountNumber string) (*model.Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Depositories) > 0 {
		return r.Depositories[0], nil
	}
	return nil, nil
}

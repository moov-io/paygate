// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

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

func (r *MockDepositoryRepository) GetUserDepositories(userID string) ([]*Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Depositories, nil
}

func (r *MockDepositoryRepository) GetUserDepository(id DepositoryID, userID string) (*Depository, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Depositories) > 0 {
		return r.Depositories[0], nil
	}
	return nil, nil
}

func (r *MockDepositoryRepository) UpsertUserDepository(userID string, dep *Depository) error {
	return r.Err
}

func (r *MockDepositoryRepository) UpdateDepositoryStatus(id DepositoryID, status DepositoryStatus) error {
	r.Status = status
	return r.Err
}

func (r *MockDepositoryRepository) deleteUserDepository(id DepositoryID, userID string) error {
	return r.Err
}

func (r *MockDepositoryRepository) GetMicroDeposits(id DepositoryID) ([]*MicroDeposit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.MicroDeposits, nil
}

func (r *MockDepositoryRepository) getMicroDepositsForUser(id DepositoryID, userID string) ([]*MicroDeposit, error) {
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

func (r *MockDepositoryRepository) LookupMicroDepositFromReturn(id DepositoryID, amount *Amount) (*MicroDeposit, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.MicroDeposits) > 0 {
		return r.MicroDeposits[0], nil
	}
	return nil, nil
}

func (r *MockDepositoryRepository) SetReturnCode(id DepositoryID, amount Amount, returnCode string) error {
	r.ReturnCode = returnCode
	return r.Err
}

func (r *MockDepositoryRepository) InitiateMicroDeposits(id DepositoryID, userID string, microDeposit []*MicroDeposit) error {
	return r.Err
}

func (r *MockDepositoryRepository) confirmMicroDeposits(id DepositoryID, userID string, amounts []Amount) error {
	return r.Err
}

func (r *MockDepositoryRepository) GetMicroDepositCursor(batchSize int) *MicroDepositCursor {
	return r.Cur
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package originators

import (
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockRepository struct {
	Originators []*model.Originator
	Err         error
}

func (r *MockRepository) getUserOriginators(userID id.User) ([]*model.Originator, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Originators, nil
}

func (r *MockRepository) GetUserOriginator(id model.OriginatorID, userID id.User) (*model.Originator, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Originators) > 0 {
		return r.Originators[0], nil
	}
	return nil, nil
}

func (r *MockRepository) createUserOriginator(userID id.User, req originatorRequest) (*model.Originator, error) {
	if len(r.Originators) > 0 {
		return r.Originators[0], nil
	}
	return nil, nil
}

func (r *MockRepository) updateUserOriginator(userID id.User, orig *model.Originator) error {
	return r.Err
}

func (r *MockRepository) deleteUserOriginator(id model.OriginatorID, userID id.User) error {
	return r.Err
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package receivers

import (
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockRepository struct {
	Receivers []*model.Receiver
	Err       error
}

func (r *MockRepository) getUserReceivers(userID id.User) ([]*model.Receiver, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Receivers, nil
}

func (r *MockRepository) GetUserReceiver(id model.ReceiverID, userID id.User) (*model.Receiver, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if len(r.Receivers) > 0 {
		return r.Receivers[0], nil
	}
	return nil, nil
}

func (r *MockRepository) UpdateReceiverStatus(id model.ReceiverID, status model.ReceiverStatus) error {
	return r.Err
}

func (r *MockRepository) UpsertUserReceiver(userID id.User, receiver *model.Receiver) error {
	return r.Err
}

func (r *MockRepository) deleteUserReceiver(id model.ReceiverID, userID id.User) error {
	return r.Err
}

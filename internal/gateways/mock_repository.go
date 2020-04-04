// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gateways

import (
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type MockRepository struct {
	Gateway *model.Gateway
	Err     error
}

func (r *MockRepository) GetUserGateway(userID id.User) (*model.Gateway, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Gateway, nil
}

func (r *MockRepository) createUserGateway(userID id.User, req gatewayRequest) (*model.Gateway, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Gateway, nil
}

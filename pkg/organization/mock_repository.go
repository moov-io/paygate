// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organization

import "github.com/moov-io/paygate/pkg/client"

type MockRepository struct {
	Config *client.OrgConfig
	Err    error
}

func (r *MockRepository) GetConfig(orgID string) (*client.OrgConfig, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Config, nil
}

func (r *MockRepository) UpdateConfig(orgID string, cfg *client.OrgConfig) (*client.OrgConfig, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return cfg, nil
}

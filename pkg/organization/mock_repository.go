// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organization

type MockRepository struct {
	Config *Config
	Err    error
}

func (r *MockRepository) GetConfig(orgID string) (*Config, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Config, nil
}

func (r *MockRepository) UpdateConfig(orgID string, companyID string) (bool, error) {
	return true, nil
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package namespace

type MockRepository struct {
	Config *Config
	Err    error
}

func (r *MockRepository) GetConfig(namespace string) (*Config, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Config, nil
}

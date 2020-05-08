// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"github.com/moov-io/paygate/pkg/client"
)

type MockRepository struct {
	CompanyIdentification string

	Err error
}

func (r *MockRepository) Create(userID string, companyIdentification string, tenant client.Tenant) error {
	return r.Err
}

func (r *MockRepository) GetCompanyIdentification(tenantID string) (string, error) {
	if r.Err != nil {
		return "", r.Err
	}
	return r.CompanyIdentification, nil
}

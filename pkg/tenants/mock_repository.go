// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"github.com/moov-io/paygate/pkg/client"
)

type MockRepository struct {
	Tenants               []client.Tenant
	CompanyIdentification string

	Err error
}

func (r *MockRepository) Create(tenant client.Tenant, companyIdentification string) error {
	return r.Err
}

func (r *MockRepository) List(tenantID string) ([]client.Tenant, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Tenants, nil
}

func (r *MockRepository) GetCompanyIdentification(tenantID string) (string, error) {
	if r.Err != nil {
		return "", r.Err
	}
	return r.CompanyIdentification, nil
}

func (r *MockRepository) UpdateTenant(tenantID string, req client.UpdateTenant) error {
	return r.Err
}

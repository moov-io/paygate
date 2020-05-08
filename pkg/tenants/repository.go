// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"database/sql"

	"github.com/moov-io/paygate/pkg/client"
)

type Repository interface {
	Create(userID string, companyIdentification string, tenant client.Tenant) error

	GetCompanyIdentification(tenantID string) (string, error)
}

func NewRepo(db *sql.DB) Repository {
	return &sqlRepo{db: db}
}

// create table tenants(tenant_id primary key, user_id, name, primary_customer, company_identification, created_at datetime, deleted_at datetime);

type sqlRepo struct {
	db *sql.DB
}

func (r *sqlRepo) Create(userID string, companyIdentification string, tenant client.Tenant) error {
	query := `insert into tenants (tenant_id, user_id, name, primary_customer, company_identification, created_at) values (?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(tenant.TenantID, userID, tenant.Name, tenant.PrimaryCustomer, companyIdentification)
	return err
}

func (r *sqlRepo) GetCompanyIdentification(tenantID string) (string, error) {
	query := `select company_identification from tenants where tenant_id = ? and deleted_at is null limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	var companyIdentification string
	if err := stmt.QueryRow(tenantID).Scan(&companyIdentification); err != nil {
		return "", err
	}
	return companyIdentification, nil
}

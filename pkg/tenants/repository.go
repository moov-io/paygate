// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/moov-io/paygate/pkg/client"
)

type Repository interface {
	Create(userID string, companyIdentification string, tenant client.Tenant) error
	List(userID string) ([]client.Tenant, error)

	GetCompanyIdentification(tenantID string) (string, error)

	UpdateTenant(tenantID string, req client.UpdateTenant) error
}

func NewRepo(db *sql.DB) Repository {
	return &sqlRepo{db: db}
}

type sqlRepo struct {
	db *sql.DB
}

func (r *sqlRepo) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *sqlRepo) Create(userID string, companyIdentification string, tenant client.Tenant) error {
	query := `insert into tenants (tenant_id, user_id, name, primary_customer, company_identification, created_at) values (?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(tenant.TenantID, userID, tenant.Name, tenant.PrimaryCustomer, companyIdentification, time.Now())
	return err
}

func (r *sqlRepo) List(userID string) ([]client.Tenant, error) {
	query := `select tenant_id, name, primary_customer from tenants where user_id = ? and deleted_at is null;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []client.Tenant
	for rows.Next() {
		var tenant client.Tenant
		if err := rows.Scan(&tenant.TenantID, &tenant.Name, &tenant.PrimaryCustomer); err != nil {
			return nil, fmt.Errorf("list: tenantID=%s error=%v", tenant.TenantID, err)
		}
		out = append(out, tenant)
	}
	return out, nil
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

func (r *sqlRepo) UpdateTenant(tenantID string, req client.UpdateTenant) error {
	query := `update tenants set name = ? where tenant_id = ? and deleted_at is null;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(req.Name, tenantID)
	return err
}

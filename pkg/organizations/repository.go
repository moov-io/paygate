// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organizations

import (
	"database/sql"
	"time"

	"github.com/moov-io/paygate/pkg/client"
)

type Repository interface {
	getOrganizations(tenantID string) ([]client.Organization, error)
	createOrganization(tenantID string, org client.Organization) error
	updateOrganizationName(orgID, name string) error
}

func NewRepo(db *sql.DB) Repository {
	return &sqlRepo{db: db}
}

type sqlRepo struct {
	db *sql.DB
}

func (r *sqlRepo) getOrganizations(tenantID string) ([]client.Organization, error) {
	query := `select organization_id, name, tenant_id, primary_customer from organizations
where tenant_id = ? and deleted_at is null and deleted_at is null;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(tenantID)
	if err != nil {
		return nil, err
	}

	var out []client.Organization
	for rows.Next() {
		var org client.Organization
		if err := rows.Scan(&org.OrganizationID, &org.Name, &org.TenantID, &org.PrimaryCustomer); err != nil {
			return nil, err
		}
		out = append(out, org)
	}
	return out, nil
}

func (r *sqlRepo) createOrganization(tenantID string, org client.Organization) error {
	query := `insert into organizations (organization_id, tenant_id, name, primary_customer, created_at) values (?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(org.OrganizationID, tenantID, org.Name, org.PrimaryCustomer, time.Now())
	return err
}

func (r *sqlRepo) updateOrganizationName(orgID, name string) error {
	query := `update organizations set name = ? where organization_id = ?;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(name, orgID)
	return err
}

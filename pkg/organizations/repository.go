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
	getOrganizations(userID string) ([]client.Organization, error)
	createOrganization(userID string, org client.Organization) error
	updateOrganizationName(orgID, name string) error
}

func NewRepo(db *sql.DB) Repository {
	return &sqlRepo{db: db}
}

type sqlRepo struct {
	db *sql.DB
}

func (r *sqlRepo) getOrganizations(userID string) ([]client.Organization, error) {
	query := `select o.organization_id, o.name, ts.tenant_id, o.primary_customer from organizations as o
inner join tenants_organizations as ts on o.organization_id = ts.organization_id
where o.user_id = ? and o.deleted_at is null and ts.deleted_at is null;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
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

func (r *sqlRepo) createOrganization(userID string, org client.Organization) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	query := `insert into organizations (organization_id, user_id, name, primary_customer, created_at) values (?, ?, ?, ?, ?);`
	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(org.OrganizationID, userID, org.Name, org.PrimaryCustomer, time.Now())
	if err != nil {
		tx.Rollback()
		return err
	}

	query = `replace into tenants_organizations(tenant_id, organization_id, created_at, deleted_at) values (?, ?, ?, null);`
	stmt, err = tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(org.TenantID, org.OrganizationID, time.Now())
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
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

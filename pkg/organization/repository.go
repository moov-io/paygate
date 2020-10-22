// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organization

import (
	"database/sql"
	"fmt"
	"github.com/moov-io/paygate/pkg/client"
)

type Repository interface {
	GetConfig(orgID string) (*client.OrganizationConfiguration, error)
	UpdateConfig(orgID string, cfg *client.OrganizationConfiguration) (*client.OrganizationConfiguration, error)
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

func (r *sqlRepo) GetConfig(orgID string) (*client.OrganizationConfiguration, error) {
	query := `select company_identification from organization_configs where organization = ? limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var cfg client.OrganizationConfiguration
	if err := stmt.QueryRow(orgID).Scan(&cfg.CompanyIdentification); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *sqlRepo) UpdateConfig(orgID string, cfg *client.OrganizationConfiguration) (*client.OrganizationConfiguration, error) {
	query := `update organization_configs set company_identification = ? where organization = ? limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("config: organization or company does not belong: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(cfg.CompanyIdentification, orgID)
	if err != nil {
		return nil, fmt.Errorf("config: issue updating config: %v", err)
	}
	return cfg, nil
}

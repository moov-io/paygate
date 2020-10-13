// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organization

import (
	"database/sql"
)

type Repository interface {
	GetConfig(orgID string) (*Config, error)
	UpdateConfig(orgID string, companyID string) (bool, error)
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

func (r *sqlRepo) GetConfig(orgID string) (*Config, error) {
	query := `select company_identification from organization_configs where organization = ? limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var cfg Config
	if err := stmt.QueryRow(orgID).Scan(&cfg.CompanyIdentification); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *sqlRepo) UpdateConfig(orgID string, companyID string) (bool, error) {
	query := `update organization_configs set company_identification = ? where organization = ? limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return false, err
	}
	defer stmt.Close()

	_, err = stmt.Exec(companyID, orgID)
	if err != nil {
		return false, err
	}
	return true, nil
}

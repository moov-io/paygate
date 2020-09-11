// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package namespace

import (
	"database/sql"
)

type Repository interface {
	GetConfig(namespace string) (*Config, error)
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

func (r *sqlRepo) GetConfig(namespace string) (*Config, error) {
	query := `select company_identification from namespace_configs where namespace = ? limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var cfg Config
	if err := stmt.QueryRow(namespace).Scan(&cfg.CompanyIdentification); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

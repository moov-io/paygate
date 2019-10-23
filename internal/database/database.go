// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/lopezator/migrator"

	"github.com/moov-io/paygate/internal/config"
)

func New(logger log.Logger, cfg *config.Config) (*sql.DB, error) {
	logger.Log("database", fmt.Sprintf("looking for %s database provider", cfg.DatabaseType))
	switch strings.ToLower(cfg.DatabaseType) {
	case "sqlite", "":
		return sqliteConnection(logger, getSqlitePath(cfg)).Connect()
	case "mysql":
		return mysqlConnection(logger, cfg).Connect()
	}
	return nil, fmt.Errorf("unknown database type %q", cfg.DatabaseType)
}

func execsql(name, raw string) *migrator.MigrationNoTx {
	return &migrator.MigrationNoTx{
		Name: name,
		Func: func(db *sql.DB) error {
			_, err := db.Exec(raw)
			return err
		},
	}
}

// UniqueViolation returns true when the provided error matches a database error
// for duplicate entries (violating a unique table constraint).
func UniqueViolation(err error) bool {
	return MySQLUniqueViolation(err) || SqliteUniqueViolation(err)
}

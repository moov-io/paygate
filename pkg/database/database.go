// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"context"
	"database/sql"

	"github.com/moov-io/paygate/pkg/config"

	"github.com/lopezator/migrator"
	"github.com/moov-io/base/log"
)

// New establishes a database connection according to the type and environmental
// variables for that specific database.
func New(ctx context.Context, logger log.Logger, cfg config.Database) (*sql.DB, error) {
	if cfg.MySQL != nil {
		logger.Log("setting up mysql database provider")
		return mysqlConnection(logger, cfg.MySQL.Username, cfg.MySQL.GetPassword(), cfg.MySQL.Address, cfg.MySQL.Database).Connect(ctx)
	}

	logger.Log("setting up sqlite database provider")
	return sqliteConnection(logger, cfg.SQLite.Path).Connect(ctx)
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

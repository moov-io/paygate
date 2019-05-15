// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
)

type db interface {
	Connect() (*sql.DB, error)
}

func New(logger log.Logger, _type string) (*sql.DB, error) {
	switch strings.ToLower(_type) {
	case "sqlite", "":
		return sqliteConnection(logger, getSqlitePath()).Connect()
	case "mysql":
		return mysqlConnection(logger, os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_PASSWORD"), os.Getenv("MYSQL_ADDRESS"), os.Getenv("MYSQL_DATABASE")).Connect()
	}
	return nil, fmt.Errorf("Unknown database type %q", _type)
}

// UniqueViolation returns true when the provided error matches a database error
// for duplicate entries (violating a unique table constraint).
func UniqueViolation(err error) bool {
	return MySQLUniqueViolation(err) || SqliteUniqueViolation(err)
}

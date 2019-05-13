// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
)

type db interface {
	Connect() (*sql.DB, error)
}

func New(logger log.Logger, _type string) (*sql.DB, error) {
	switch strings.ToLower(_type) {
	case "sqlite", "":
		return createSqliteConnection(logger, getSqlitePath()).Connect()
	case "mysql":
		return createMysqlConnection(logger).Connect()
	}
	return nil, fmt.Errorf("Unknown database type %q", _type)
}

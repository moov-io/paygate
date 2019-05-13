// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"database/sql"

	"github.com/go-kit/kit/log"
	_ "github.com/go-sql-driver/mysql"
)

type mysql struct {
}

func (my *mysql) Connect() (*sql.DB, error) {
	return nil, nil
}

func createMysqlConnection(logger log.Logger) *mysql { // TODO(adam): accept other params
	// username:password@protocol(address)/dbname?param=value
	return &mysql{}
}

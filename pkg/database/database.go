// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"context"
	"database/sql"

	"github.com/moov-io/paygate/pkg/config"

	"github.com/moov-io/base/database"
	"github.com/moov-io/base/log"
)

func New(ctx context.Context, logger log.Logger, cfg config.Database) (*sql.DB, error) {
	dbConfig := database.DatabaseConfig{}

	if cfg.MySQL != nil {
		dbConfig.MySQL = &database.MySQLConfig{
			Address:  cfg.MySQL.Address,
			User:     cfg.MySQL.Username,
			Password: cfg.MySQL.GetPassword(),
		}
		dbConfig.DatabaseName = cfg.MySQL.Database
	} else {
		dbConfig.SQLite = &database.SQLiteConfig{
			Path: cfg.SQLite.Path,
		}
	}

	return database.NewAndMigrate(ctx, logger, dbConfig)
}

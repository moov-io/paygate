// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func getSqlitePath() string {
	path := os.Getenv("SQLITE_DB_PATH")
	if path == "" || strings.Contains(path, "..") {
		// set default if empty or trying to escape
		// don't filepath.ABS to avoid full-fs reads
		path = "paygate.db"
	}
	return path
}

func createSqliteConnection(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		err = fmt.Errorf("problem opening sqlite3 file %s: %v", path, err)
		logger.Log("sqlite", err)
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("problem with Ping against *sql.DB %s: %v", path, err)
	}
	return db, nil
}

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"testing"

	"github.com/go-kit/kit/log"
)

func TestSQLite__basic(t *testing.T) {
	db := CreateTestSqliteDB(t)
	defer db.Close()

	res, err := db.DB.Query("select 1")
	if err != nil {
		t.Error(err.Error())
	}
	res.Close()
}

func TestSQLite__getSqlitePath(t *testing.T) {
	if v := getSqlitePath(); v != "paygate.db" {
		t.Errorf("got %s", v)
	}
}

func TestSQLite__sqliteConnection(t *testing.T) {
	db := sqliteConnection(log.NewNopLogger(), "")
	if db == nil {
		t.Fatal("nil *sqlite")
	}
	if len(db.migrations) == 0 {
		t.Error("expected SQLite migrations")
	}
}

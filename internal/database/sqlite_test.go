// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"errors"
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

func TestSqliteUniqueViolation(t *testing.T) {
	err := errors.New(`problem upserting depository="7d676c65eccd48090ff238a0d5e35eb6126c23f2", userId="80cfe1311d9eb7659d02cba9ee6cb04ed3739a85": UNIQUE constraint failed: depositories.depository_id`)
	if !UniqueViolation(err) {
		t.Error("should have matched unique violation")
	}
}

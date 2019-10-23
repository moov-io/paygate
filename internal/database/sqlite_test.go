// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"errors"
	"runtime"
	"testing"

	"github.com/moov-io/paygate/internal/config"

	"github.com/go-kit/kit/log"
)

func TestSQLite__basic(t *testing.T) {
	db := CreateTestSqliteDB(t)
	defer db.Close()

	if err := db.DB.Ping(); err != nil {
		t.Fatal(err)
	}

	if runtime.GOOS == "windows" {
		t.Skip("/dev/null doesn't exist on Windows")
	}

	// error case
	s := sqliteConnection(log.NewNopLogger(), "/tmp/path/doesnt/exist")

	conn, _ := s.Connect()
	if err := conn.Ping(); err == nil {
		t.Error("expected error")
	}
	conn.Close()
}

func TestSQLite__getSqlitePath(t *testing.T) {
	cfg := config.Config{}
	if v := getSqlitePath(&cfg); v != "paygate.db" {
		t.Errorf("got %s", v)
	}
}

func TestSqliteUniqueViolation(t *testing.T) {
	err := errors.New(`problem upserting depository="7d676c65eccd48090ff238a0d5e35eb6126c23f2", userId="80cfe1311d9eb7659d02cba9ee6cb04ed3739a85": UNIQUE constraint failed: depositories.depository_id`)
	if !UniqueViolation(err) {
		t.Error("should have matched unique violation")
	}
}

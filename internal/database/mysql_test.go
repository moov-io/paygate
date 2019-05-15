// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"errors"
	"testing"

	"github.com/go-kit/kit/log"
)

func TestMySQL__basic(t *testing.T) {
	db := CreateTestMySQLDB(t)
	defer db.Close()

	if err := db.DB.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQL__mysqlConnection(t *testing.T) {
	db := mysqlConnection(log.NewNopLogger(), "", "", "", "")
	if db == nil {
		t.Fatal("nil *mysql")
	}
	if len(db.migrations) == 0 {
		t.Error("expected MySQL migrations")
	}
}

func TestMySQLUniqueViolation(t *testing.T) {
	err := errors.New(`problem upserting depository="282f6ffcd9ba5b029afbf2b739ee826e22d9df3b", userId="f25f48968da47ef1adb5b6531a1c2197295678ce": Error 1062: Duplicate entry '282f6ffcd9ba5b029afbf2b739ee826e22d9df3b' for key 'PRIMARY'`)
	if !UniqueViolation(err) {
		t.Error("should have matched unique violation")
	}
}

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
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

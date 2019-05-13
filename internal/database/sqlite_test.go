// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"testing"
)

func TestSqlite__basic(t *testing.T) {
	r, err := CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	res, err := r.DB.Query("select 1")
	if err != nil {
		t.Error(err.Error())
	}
	res.Close()
}

func TestSqlite__getSqlitePath(t *testing.T) {
	if v := getSqlitePath(); v != "paygate.db" {
		t.Errorf("got %s", v)
	}
}

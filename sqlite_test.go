// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"io/ioutil"
	"os"
	"testing"
)

func TestSqlite__basic(t *testing.T) {
	// setup temp database
	f, err := ioutil.TempFile("", "auth-sqlite3-test")
	if err != nil {
		t.Fatal(err.Error())
	}
	defer os.Remove(f.Name())

	db, err := sql.Open("sqlite3", f.Name()+".db")
	if err != nil {
		t.Fatal(err.Error())
	}

	// sanity spec
	res, err := db.Query("select 1")
	if err != nil {
		t.Error(err.Error())
	}
	res.Close()
}

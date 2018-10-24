// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/kit/log"
)

type testSqliteDB struct {
	db *sql.DB

	dir string // temp dir created for sqlite files
}

func (r *testSqliteDB) close() error {
	if err := r.db.Close(); err != nil {
		return err
	}
	return os.RemoveAll(r.dir)
}

// createTestSqliteDB returns a testSqliteDB which can be used in tests
// as a clean sqlite database. All migrations are ran on the db before.
//
// Callers should call close on the returned *testSqliteDB.
func createTestSqliteDB() (*testSqliteDB, error) {
	dir, err := ioutil.TempDir("", "paygate-sqlite")
	if err != nil {
		return nil, err
	}

	db, err := createSqliteConnection(filepath.Join(dir, "paygate.db"))
	if err != nil {
		return nil, err
	}

	logger := log.NewLogfmtLogger(ioutil.Discard)
	if err := migrate(db, logger); err != nil {
		return nil, err
	}

	return &testSqliteDB{db, dir}, nil
}

func TestSqlite__basic(t *testing.T) {
	r, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()

	res, err := r.db.Query("select 1")
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

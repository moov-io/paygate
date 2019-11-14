// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
)

func TestDepository__grabEncryptableDepositories(t *testing.T) {
	logger := log.NewNopLogger()

	db, err := sql.Open("sqlite3", "../paygate.db")
	if err != nil {
		t.Fatal(err)
	}

	repo := NewDepositoryRepo(logger, db)
	rows, err := grabEncryptableDepositories(logger, repo, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) < 2 {
		t.Logf("found %d rows", len(rows))
		return
	}

	for i := range rows[:2] {
		t.Logf("rows[%d]=%#v", i, rows[i])
	}
}

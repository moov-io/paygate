// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
)

func TestAttempter__available(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	depID := DepositoryID(base.ID())
	at := &Attemper{db: db.DB, logger: log.NewNopLogger(), maxAttempts: 2}

	if !at.Available(depID) {
		t.Errorf("expected to have attempts")
	}

	// write one attempt
	if err := at.Record(depID, "0.12,0.32,0.44"); err != nil {
		t.Errorf("problem recording micro-deposits: %v", err)
	}
	if !at.Available(depID) {
		t.Error("expected to have attempts")
	}

	// write a success
	if err := at.Record(depID, "0.11,0.32,0.43"); err != nil {
		t.Errorf("problem recording micro-deposits: %v", err)
	}
	if at.Available(depID) {
		t.Error("expected no attempts")
	}

	// a new DepositoryID has attempts left
	if !at.Available(DepositoryID(base.ID())) {
		t.Error("expected to have attempts")
	}
}

func TestAttempter__failed(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	depID := DepositoryID(base.ID())
	at := &Attemper{db: db.DB, logger: log.NewNopLogger(), maxAttempts: 2}

	if !at.Available(depID) {
		t.Errorf("expected to have attempts")
	}

	if err := at.Record(depID, "0.12,0.32,0.44"); err != nil {
		t.Errorf("problem recording micro-deposits: %v", err)
	}
	if err := at.Record(depID, "0.12,0.32,0.44"); err != nil {
		t.Errorf("problem recording micro-deposits: %v", err)
	}
	if at.Available(depID) {
		t.Error("expected no attempts left")
	}
}

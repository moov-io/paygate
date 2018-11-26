// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/go-kit/kit/log"
)

func TestMicroDeposits__repository(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteDepositoryRepo{db.db, log.NewNopLogger()}

	id, userId := DepositoryID(nextID()), nextID()

	// ensure none exist on an empty slate
	amounts, err := r.getMicroDeposits(id, userId)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(amounts); n != 0 {
		t.Errorf("got %d micro deposits", n)
	}

	// write deposits
	if err := r.initiateMicroDeposits(id, userId, fixedMicroDepositAmounts); err != nil {
		t.Fatal(err)
	}

	// Confirm (success)
	if err := r.confirmMicroDeposits(id, userId, fixedMicroDepositAmounts); err != nil {
		t.Error(err)
	}

	// Confirm (incorrect amounts)
	amt, _ := NewAmount("USD", "0.01")
	if err := r.confirmMicroDeposits(id, userId, []Amount{*amt}); err == nil {
		t.Error("expected error, but got none")
	}

	// Confirm (empty guess)
	if err := r.confirmMicroDeposits(id, userId, nil); err == nil {
		t.Error("expected error, but got none")
	}
}

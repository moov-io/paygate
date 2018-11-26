// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
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

func TestMicroDeposits__routes(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteDepositoryRepo{db.db, log.NewNopLogger()}
	id, userId := DepositoryID(nextID()), nextID()

	// Write depository
	dep := &Depository{
		ID:            id,
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       time.Now().Add(-1 * time.Second),
	}
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Fatal(err)
	}

	handler := mux.NewRouter()
	addDepositoryRoutes(handler, r)

	// inititate our micro deposits
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits", id), nil)
	req.Header.Set("x-user-id", userId)
	handler.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("initiate got %d status", w.Code)
	}

	// confirm our deposits
	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(confirmDepositoryRequest{
		Amounts: []string{zzone.String(), zzthree.String()}, // from microDeposits.go
	})
	if err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits/confirm", id), &buf)
	req.Header.Set("x-user-id", userId)
	handler.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("confirm got %d status", w.Code)
	}
}

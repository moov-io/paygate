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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestMicroDeposits__repository(t *testing.T) {
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}

	id, userId := DepositoryID(base.ID()), base.ID()

	// ensure none exist on an empty slate
	amounts, err := r.getMicroDeposits(id, userId)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(amounts); n != 0 {
		t.Errorf("got %d micro deposits", n)
	}

	// write deposits
	var microDeposits []microDeposit
	for i := range fixedMicroDepositAmounts {
		microDeposits = append(microDeposits, microDeposit{
			amount: fixedMicroDepositAmounts[i],
		})
	}
	if err := r.initiateMicroDeposits(id, userId, microDeposits); err != nil {
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
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}
	id, userId := DepositoryID(base.ID()), base.ID()

	eventRepo := &sqliteEventRepo{db.DB, log.NewNopLogger()}

	// Write depository
	dep := &Depository{
		ID:            id,
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "121042882",
		AccountNumber: "151",
		Status:        DepositoryUnverified, // status is checked in initiateMicroDeposits
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Fatal(err)
	}

	handler := mux.NewRouter()
	fedClient := &testFEDClient{}
	ofacClient := &testOFACClient{}
	addDepositoryRoutes(log.NewNopLogger(), handler, fedClient, ofacClient, r, eventRepo)

	// Bring up a test ACH instance
	_, _, server := achclient.MockClientServer("micro-deposits", func(r *mux.Router) {
		achclient.AddCreateRoute(nil, r)
		achclient.AddValidateRoute(r)
	})
	defer server.Close()

	// Set ACH_ENDPOINT to override the achclient.New call
	os.Setenv("ACH_ENDPOINT", server.URL)

	// inititate our micro deposits
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits", id), nil)
	req.Header.Set("x-user-id", userId)
	handler.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusCreated {
		if !strings.Contains(w.Body.String(), ":8080: connect: connection refused") {
			t.Errorf("initiate got %d status: %v", w.Code, w.Body.String())
		}
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
		t.Errorf("confirm got %d status: %v", w.Code, w.Body.String())
	}
}

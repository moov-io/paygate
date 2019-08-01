// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	accounts "github.com/moov-io/accounts/client"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func makeTestODFIAccount() *odfiAccount {
	return &odfiAccount{
		routingNumber: "121042882", // set as ODFIIdentification in PPD batches (used in tests)
		accountId:     "odfi-account",
	}
}

func TestODFIAccount(t *testing.T) {
	accountsClient := &testAccountsClient{}
	odfi := &odfiAccount{
		client:        accountsClient,
		accountNumber: "",
		routingNumber: "",
		accountType:   Savings,
		accountId:     "accountId",
	}

	orig, dep := odfi.metadata()
	if orig.ID != "odfi" {
		t.Errorf("originator: %#v", orig)
	}
	if string(dep.ID) != "odfi" {
		t.Errorf("depository: %#v", dep)
	}

	if accountId, err := odfi.getID("", "userId"); accountId != "accountId" || err != nil {
		t.Errorf("accountId=%s error=%v", accountId, err)
	}
	odfi.accountId = "" // unset so we make the AccountsClient call
	accountsClient.accounts = []accounts.Account{
		{
			Id: "accountId2",
		},
	}
	if accountId, err := odfi.getID("", "userId"); accountId != "accountId2" || err != nil {
		t.Errorf("accountId=%s error=%v", accountId, err)
	}
	if odfi.accountId != "accountId2" {
		t.Errorf("odfi.accountId=%s", odfi.accountId)
	}

	// error on AccountsClient call
	odfi.accountId = ""
	accountsClient.err = errors.New("bad")
	if accountId, err := odfi.getID("", "userId"); accountId != "" || err == nil {
		t.Errorf("expected error accountId=%s", accountId)
	}

	// on nil AccountsClient expect an error
	odfi.client = nil
	if accountId, err := odfi.getID("", "userId"); accountId != "" || err == nil {
		t.Errorf("expcted error accountId=%s", accountId)
	}
}

func TestMicroDeposits__microDepositAmounts(t *testing.T) {
	for i := 0; i < 100; i++ {
		amounts := microDepositAmounts()
		if len(amounts) != 3 {
			t.Errorf("got %d micro-deposit amounts", len(amounts))
		}
		if v := (amounts[0].Int() + amounts[1].Int()); v != amounts[2].Int() {
			t.Errorf("v=%d sum=%d", v, amounts[2].Int())
		}
	}
}

func TestMicroDeposits__repository(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo depositoryRepository) {
		id, userId := DepositoryID(base.ID()), base.ID()

		// ensure none exist on an empty slate
		amounts, err := repo.getMicroDeposits(id, userId)
		if err != nil {
			t.Fatal(err)
		}
		if n := len(amounts); n != 0 {
			t.Errorf("got %d micro deposits", n)
		}

		// write deposits
		var microDeposits []microDeposit
		randomAmounts := microDepositAmounts()
		for i := range randomAmounts {
			microDeposits = append(microDeposits, microDeposit{
				amount: randomAmounts[i],
			})
		}
		if err := repo.initiateMicroDeposits(id, userId, microDeposits); err != nil {
			t.Fatal(err)
		}
		amounts, err = repo.getMicroDeposits(id, userId)
		if err != nil || len(amounts) != 3 {
			t.Fatalf("amounts=%#v error=%v", amounts, err)
		}

		// Confirm (success)
		if err := repo.confirmMicroDeposits(id, userId, randomAmounts); err != nil {
			t.Error(err)
		}

		// Confirm (incorrect amounts)
		amt, _ := NewAmount("USD", "0.01")
		if err := repo.confirmMicroDeposits(id, userId, []Amount{*amt}); err == nil {
			t.Error("expected error, but got none")
		}

		// Confirm (empty guess)
		if err := repo.confirmMicroDeposits(id, userId, nil); err == nil {
			t.Error("expected error, but got none")
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &sqliteDepositoryRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &sqliteDepositoryRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestMicroDeposits__routes(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, db *sql.DB) {
		id, userId := DepositoryID(base.ID()), base.ID()

		depRepo := &sqliteDepositoryRepo{db, log.NewNopLogger()}
		eventRepo := &sqliteEventRepo{db, log.NewNopLogger()}

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
		if err := depRepo.upsertUserDepository(userId, dep); err != nil {
			t.Fatal(err)
		}

		accountsClient := &testAccountsClient{
			accounts: []accounts.Account{{Id: base.ID()}},
			transaction: &accounts.Transaction{
				Id: base.ID(),
			},
		}
		fedClient, ofacClient := &testFEDClient{}, &testOFACClient{}

		achClient, _, server := achclient.MockClientServer("micro-deposits", func(r *mux.Router) {
			achclient.AddCreateRoute(nil, r)
			achclient.AddValidateRoute(r)
		})
		defer server.Close()

		testODFIAccount := makeTestODFIAccount()

		handler := mux.NewRouter()
		addDepositoryRoutes(log.NewNopLogger(), handler, testODFIAccount, false, accountsClient, achClient, fedClient, ofacClient, depRepo, eventRepo)

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
				t.Errorf("initiate got %d status: %v", w.Code, w.Body.String()) // TODO(adam): Accountslient needs a stub
			}
		}

		// confirm our deposits
		var buf bytes.Buffer
		var request confirmDepositoryRequest
		for i := range accountsClient.postedTransactions {
			for j := range accountsClient.postedTransactions[i].Lines {
				// Only take the credit amounts (as we only need the amount from one side of the dual entry)
				if strings.EqualFold(accountsClient.postedTransactions[i].Lines[j].Purpose, "ACHCredit") {
					request.Amounts = append(request.Amounts, fmt.Sprintf("USD 0.%02d", accountsClient.postedTransactions[i].Lines[j].Amount))
				}
			}
		}
		if len(request.Amounts) != 3 {
			t.Errorf("got %d amounts", len(request.Amounts))
		}
		if err := json.NewEncoder(&buf).Encode(request); err != nil {
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

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, sqliteDB.DB)

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, mysqlDB.DB)
}

func TestMicroDeposits__markMicroDepositAsMerged(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqliteDepositoryRepo) {
		amt, _ := NewAmount("USD", "0.11")
		microDeposits := []microDeposit{
			{amount: *amt, fileId: "fileId"},
		}
		if err := repo.initiateMicroDeposits(DepositoryID("id"), "userId", microDeposits); err != nil {
			t.Fatal(err)
		}

		mc := uploadableMicroDeposit{
			depositoryId: "id",
			userId:       "userId",
			amount:       amt,
			fileId:       "fileId",
		}
		if err := repo.markMicroDepositAsMerged("filename", mc); err != nil {
			t.Fatal(err)
		}

		// Read merged_filename and verify
		query := `select merged_filename from micro_deposits where amount = 'USD 0.11' and depository_id = 'id' limit 1;`
		stmt, err := repo.db.Prepare(query)
		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()

		var mergedFilename string
		if err := stmt.QueryRow().Scan(&mergedFilename); err != nil {
			t.Fatal(err)
		}
		if mergedFilename != "filename" {
			t.Errorf("mergedFilename=%s", mergedFilename)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &sqliteDepositoryRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &sqliteDepositoryRepo{mysqlDB.DB, log.NewNopLogger()})
}

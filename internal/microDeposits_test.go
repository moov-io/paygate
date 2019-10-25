// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

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
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/ofac"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func makeTestODFIAccount() *ODFIAccount {
	return &ODFIAccount{
		routingNumber: "121042882", // set as ODFIIdentification in PPD batches (used in tests)
		accountID:     "odfi-account",
	}
}

func TestODFIAccount(t *testing.T) {
	accountsClient := &testAccountsClient{}
	odfi := &ODFIAccount{
		client:        accountsClient,
		accountNumber: "",
		routingNumber: "",
		accountType:   Savings,
		accountID:     "accountID",
	}

	orig, dep := odfi.metadata(config.Empty())
	if orig.ID != "odfi" {
		t.Errorf("originator: %#v", orig)
	}
	if string(dep.ID) != "odfi" {
		t.Errorf("depository: %#v", dep)
	}

	if accountID, err := odfi.getID("", "userID"); accountID != "accountID" || err != nil {
		t.Errorf("accountID=%s error=%v", accountID, err)
	}
	odfi.accountID = "" // unset so we make the AccountsClient call
	accountsClient.accounts = []accounts.Account{
		{
			ID: "accountID2",
		},
	}
	if accountID, err := odfi.getID("", "userID"); accountID != "accountID2" || err != nil {
		t.Errorf("accountID=%s error=%v", accountID, err)
	}
	if odfi.accountID != "accountID2" {
		t.Errorf("odfi.accountID=%s", odfi.accountID)
	}

	// error on AccountsClient call
	odfi.accountID = ""
	accountsClient.err = errors.New("bad")
	if accountID, err := odfi.getID("", "userID"); accountID != "" || err == nil {
		t.Errorf("expected error accountID=%s", accountID)
	}

	// on nil AccountsClient expect an error
	odfi.client = nil
	if accountID, err := odfi.getID("", "userID"); accountID != "" || err == nil {
		t.Errorf("expcted error accountID=%s", accountID)
	}
}

func TestMicroDeposits__json(t *testing.T) {
	amt, _ := NewAmount("USD", "1.24")
	bs, err := json.Marshal([]MicroDeposit{
		{Amount: *amt},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v := string(bs); v != `[{"amount":"USD 1.24"}]` {
		t.Error(v)
	}
}

func TestMicroDeposits__microDepositAmounts(t *testing.T) {
	for i := 0; i < 100; i++ {
		amounts := microDepositAmounts()
		if len(amounts) != 2 {
			t.Errorf("got %d micro-deposit amounts", len(amounts))
		}
		sum, _ := amounts[0].Plus(amounts[1])
		if sum.Int() != (amounts[0].Int() + amounts[1].Int()) {
			t.Errorf("amounts[0]=%s amounts[1]=%s", amounts[0].String(), amounts[1].String())
		}
	}
}

func TestMicroDeposits__confirmMicroDeposits(t *testing.T) {
	type state struct {
		guesses       []Amount
		microDeposits []*MicroDeposit
	}
	testCases := []struct {
		name               string
		state              state
		expectedErrMessage string
	}{
		{
			"There are 0 microdeposits",
			state{
				microDeposits: []*MicroDeposit{},
				guesses:       []Amount{},
			},
			"unable to confirm micro deposits, got 0 micro deposits",
		},
		{
			"There are less guesses than microdeposits",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: Amount{number: 10, symbol: "USD"}},
					{Amount: Amount{number: 4, symbol: "USD"}},
				},
				guesses: []Amount{
					{number: 10, symbol: "USD"},
				},
			},
			"incorrect amount of guesses, got 1",
		},
		{
			"There are more guesses than microdeposits",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: Amount{number: 10, symbol: "USD"}},
					{Amount: Amount{number: 4, symbol: "USD"}},
				},
				guesses: []Amount{
					{number: 10, symbol: "USD"},
					{number: 4, symbol: "USD"},
					{number: 7, symbol: "USD"},
				},
			},
			"incorrect amount of guesses, got 3",
		},
		{
			"One guess is correct, the other is wrong",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: Amount{number: 10, symbol: "USD"}},
					{Amount: Amount{number: 4, symbol: "USD"}},
				},
				guesses: []Amount{
					{number: 10, symbol: "USD"},
					{number: 7, symbol: "USD"},
				},
			},
			"incorrect micro deposit guesses",
		},
		{
			"Both guesses are wrong",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: Amount{number: 10, symbol: "USD"}},
					{Amount: Amount{number: 4, symbol: "USD"}},
				},
				guesses: []Amount{
					{number: 1, symbol: "USD"},
					{number: 7, symbol: "USD"},
				},
			},
			"incorrect micro deposit guesses",
		},
		{
			"Both guesses are correct",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: Amount{number: 10, symbol: "USD"}},
					{Amount: Amount{number: 4, symbol: "USD"}},
				},
				guesses: []Amount{
					{number: 10, symbol: "USD"},
					{number: 4, symbol: "USD"},
				},
			},
			"",
		},
		{
			"Both guesses are correct, in the opposite order",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: Amount{number: 10, symbol: "USD"}},
					{Amount: Amount{number: 4, symbol: "USD"}},
				},
				guesses: []Amount{
					{number: 4, symbol: "USD"},
					{number: 10, symbol: "USD"},
				},
			},
			"",
		},
	}

	sqlite := database.CreateTestSqliteDB(t)
	defer sqlite.Close()
	mysql := database.CreateTestMySQLDB(t)
	defer mysql.Close()
	databases := []*SQLDepositoryRepo{
		{sqlite.DB, log.NewNopLogger()},
		{mysql.DB, log.NewNopLogger()},
	}

	for _, db := range databases {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				depositoryID := DepositoryID(base.ID())
				userID := base.ID()

				if err := db.InitiateMicroDeposits(depositoryID, userID, tc.state.microDeposits); err != nil {
					t.Fatal(err)
				}

				err := db.confirmMicroDeposits(depositoryID, userID, tc.state.guesses)
				if tc.expectedErrMessage == "" {
					if err != nil {
						t.Errorf("nil was the expected result, got '%s' instead", err)
					}
				} else {
					if err == nil {
						t.Error("expected an error message, got nil instead")
					}
					if err.Error() != tc.expectedErrMessage {
						t.Errorf("'%s' was the expected error, got '%s' instead", tc.expectedErrMessage, err.Error())
					}
				}
			})
		}
	}
}

func TestMicroDeposits__insertMicroDepositVerify(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo DepositoryRepository) {
		id, userID := DepositoryID(base.ID()), base.ID()

		amt, _ := NewAmount("USD", "0.11")
		mc := &MicroDeposit{
			Amount:        *amt,
			FileID:        base.ID() + "-micro-deposit-verify",
			TransactionID: "transactionID",
		}
		mcs := []*MicroDeposit{mc}

		if err := repo.InitiateMicroDeposits(id, userID, mcs); err != nil {
			t.Fatal(err)
		}

		microDeposits, err := repo.getMicroDepositsForUser(id, userID)
		if n := len(microDeposits); err != nil || n == 0 {
			t.Fatalf("n=%d error=%v", n, err)
		}
		if m := microDeposits[0]; m.FileID != mc.FileID {
			t.Errorf("got %s", m.FileID)
		}
		if m := microDeposits[0]; m.TransactionID != mc.TransactionID {
			t.Errorf("got %s", m.TransactionID)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLDepositoryRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLDepositoryRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestMicroDeposits__initiateError(t *testing.T) {
	id, userID := DepositoryID(base.ID()), base.ID()
	depRepo := &MockDepositoryRepository{Err: errors.New("bad error")}
	router := &DepositoryRouter{
		logger:         log.NewNopLogger(),
		depositoryRepo: depRepo,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits", id), nil)
	req.Header.Set("x-user-id", userID)
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status %d: %s", w.Code, w.Body.String())
	}
}

func TestMicroDeposits__confirmError(t *testing.T) {
	id, userID := DepositoryID(base.ID()), base.ID()
	depRepo := &MockDepositoryRepository{Err: errors.New("bad error")}
	router := &DepositoryRouter{
		logger:         log.NewNopLogger(),
		depositoryRepo: depRepo,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(confirmDepositoryRequest{
		Amounts: []string{"USD 0.11"}, // doesn't matter as we error anyway
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits/confirm", id), &buf)
	req.Header.Set("x-user-id", userID)
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status %d: %s", w.Code, w.Body.String())
	}
}

func TestMicroDeposits__routes(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, db *sql.DB) {
		id, userID := DepositoryID(base.ID()), base.ID()

		depRepo := &SQLDepositoryRepo{db, log.NewNopLogger()}
		eventRepo := &SQLEventRepo{db, log.NewNopLogger()}

		// Write depository
		dep := &Depository{
			ID:            id,
			BankName:      "bank name",
			Holder:        "holder",
			HolderType:    Individual,
			Type:          Checking,
			RoutingNumber: "121042882",
			AccountNumber: "151",
			Status:        DepositoryUnverified, // status is checked in InitiateMicroDeposits
			Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
		}
		if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
			t.Fatal(err)
		}

		accountID := base.ID()
		accountsClient := &testAccountsClient{
			accounts: []accounts.Account{{ID: accountID}},
			transaction: &accounts.Transaction{
				ID: base.ID(),
			},
		}
		fedClient, ofacClient := &fed.TestClient{}, &ofac.TestClient{}

		achClient, _, server := achclient.MockClientServer("micro-deposits", func(r *mux.Router) {
			achclient.AddCreateRoute(nil, r)
			achclient.AddValidateRoute(r)
		})
		defer server.Close()

		testODFIAccount := makeTestODFIAccount()

		router := &DepositoryRouter{
			logger:         log.NewNopLogger(),
			config:         config.Empty(),
			odfiAccount:    testODFIAccount,
			accountsClient: accountsClient,
			achClient:      achClient,
			fedClient:      fedClient,
			ofacClient:     ofacClient,
			depositoryRepo: depRepo,
			eventRepo:      eventRepo,
		}
		r := mux.NewRouter()
		router.RegisterRoutes(r)

		// Set ACH_ENDPOINT to override the achclient.New call
		os.Setenv("ACH_ENDPOINT", server.URL)

		// inititate our micro deposits
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits", id), nil)
		req.Header.Set("x-user-id", userID)
		r.ServeHTTP(w, req)
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
				line := accountsClient.postedTransactions[i].Lines[j]

				if line.AccountID == accountID && strings.EqualFold(line.Purpose, "ACHCredit") {
					request.Amounts = append(request.Amounts, fmt.Sprintf("USD 0.%02d", line.Amount))
				}
			}
		}
		if len(request.Amounts) != 2 {
			t.Errorf("got %d amounts", len(request.Amounts))
		}
		if err := json.NewEncoder(&buf).Encode(request); err != nil {
			t.Fatal(err)
		}

		w = httptest.NewRecorder()
		req = httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits/confirm", id), &buf)
		req.Header.Set("x-user-id", userID)
		r.ServeHTTP(w, req)
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

func TestMicroDeposits__MarkMicroDepositAsMerged(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLDepositoryRepo) {
		amt, _ := NewAmount("USD", "0.11")
		microDeposits := []*MicroDeposit{
			{Amount: *amt, FileID: "fileID"},
		}
		if err := repo.InitiateMicroDeposits(DepositoryID("id"), "userID", microDeposits); err != nil {
			t.Fatal(err)
		}

		mc := UploadableMicroDeposit{
			DepositoryID: "id",
			UserID:       "userID",
			Amount:       amt,
			FileID:       "fileID",
		}
		if err := repo.MarkMicroDepositAsMerged("filename", mc); err != nil {
			t.Fatal(err)
		}

		// Read merged_filename and verify
		mergedFilename, err := ReadMergedFilename(repo, amt, DepositoryID(mc.DepositoryID))
		if err != nil {
			t.Fatal(err)
		}
		if mergedFilename != "filename" {
			t.Errorf("mergedFilename=%s", mergedFilename)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLDepositoryRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLDepositoryRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestMicroDepositCursor__next(t *testing.T) {
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	depRepo := &SQLDepositoryRepo{sqliteDB.DB, log.NewNopLogger()}
	cur := depRepo.GetMicroDepositCursor(2)

	microDeposits, err := cur.Next()
	if len(microDeposits) != 0 || err != nil {
		t.Fatalf("microDeposits=%#v error=%v", microDeposits, err)
	}

	// Write a micro-deposit
	amt, _ := NewAmount("USD", "0.11")
	if err := depRepo.InitiateMicroDeposits(DepositoryID("id"), "userID", []*MicroDeposit{{Amount: *amt, FileID: "fileID"}}); err != nil {
		t.Fatal(err)
	}
	// our cursor should return this micro-deposit now since there's no mergedFilename
	microDeposits, err = cur.Next()
	if len(microDeposits) != 1 || err != nil {
		t.Fatalf("microDeposits=%#v error=%v", microDeposits, err)
	}
	if microDeposits[0].DepositoryID != "id" || microDeposits[0].Amount.String() != "USD 0.11" {
		t.Errorf("microDeposits[0]=%#v", microDeposits[0])
	}
	mc := microDeposits[0] // save for later

	// verify calling our cursor again returns nothing
	microDeposits, err = cur.Next()
	if len(microDeposits) != 0 || err != nil {
		t.Fatalf("microDeposits=%#v error=%v", microDeposits, err)
	}

	// mark the micro-deposit as merged (via merged_filename) and re-create the cursor to expect nothing returned in Next()
	cur = depRepo.GetMicroDepositCursor(2)
	if err := depRepo.MarkMicroDepositAsMerged("filename", mc); err != nil {
		t.Fatal(err)
	}
	microDeposits, err = cur.Next()
	if len(microDeposits) != 0 || err != nil {
		t.Fatalf("microDeposits=%#v error=%v", microDeposits, err)
	}

	// verify merged_filename
	filename, err := ReadMergedFilename(depRepo, mc.Amount, DepositoryID(mc.DepositoryID))
	if err != nil {
		t.Fatal(err)
	}
	if filename != "filename" {
		t.Errorf("mc=%#v", mc)
	}
}

func TestMicroDeposits__addMicroDeposit(t *testing.T) {
	amt, _ := NewAmount("USD", "0.28")

	ed := ach.NewEntryDetail()
	ed.TransactionCode = ach.CheckingCredit
	ed.TraceNumber = "123"
	ed.Amount = 12 // $0.12

	bh := ach.NewBatchHeader()
	bh.StandardEntryClassCode = "PPD"
	batch, err := ach.NewBatch(bh)
	if err != nil {
		t.Fatal(err)
	}
	batch.AddEntry(ed)

	file := ach.NewFile()
	file.AddBatch(batch)

	if err := addMicroDeposit(file, *amt); err != nil {
		t.Fatal(err)
	}
	if len(file.Batches) != 1 || len(file.Batches[0].GetEntries()) != 2 {
		t.Fatalf("file.Batches[0]=%#v", file.Batches[0])
	}

	ed = file.Batches[0].GetEntries()[1]
	if ed.Amount != amt.Int() {
		t.Errorf("got ed.Amount=%d", ed.Amount)
	}

	// bad path
	if err := addMicroDeposit(nil, *amt); err == nil {
		t.Error("expected error")
	}
}

func TestMicroDeposits__addMicroDepositWithdraw(t *testing.T) {
	ed := ach.NewEntryDetail()
	ed.TransactionCode = ach.CheckingCredit
	ed.TraceNumber = "123"
	ed.Amount = 12 // $0.12

	bh := ach.NewBatchHeader()
	bh.StandardEntryClassCode = "PPD"
	batch, err := ach.NewBatch(bh)
	if err != nil {
		t.Fatal(err)
	}
	batch.AddEntry(ed)

	file := ach.NewFile()
	file.AddBatch(batch)

	withdrawAmount, _ := NewAmount("USD", "0.14") // not $0.12 on purpose

	// nil, so expect no changes
	if err := addMicroDepositWithdraw(nil, withdrawAmount); err == nil {
		t.Fatal("expected error")
	}
	if len(file.Batches) != 1 || len(file.Batches[0].GetEntries()) != 1 {
		t.Fatalf("file.Batches[0]=%#v", file.Batches[0])
	}

	// add reversal batch
	if err := addMicroDepositWithdraw(file, withdrawAmount); err != nil {
		t.Fatal(err)
	}

	// verify
	if len(file.Batches) != 1 {
		t.Fatalf("file.Batches=%#v", file.Batches)
	}
	entries := file.Batches[0].GetEntries()
	if len(entries) != 2 {
		t.Fatalf("entries=%#v", entries)
	}
	if entries[0].TransactionCode-5 != entries[1].TransactionCode {
		t.Errorf("entries[0].TransactionCode=%d entries[1].TransactionCode=%d", entries[0].TransactionCode, entries[1].TransactionCode)
	}
	if entries[0].Amount != 12 {
		t.Errorf("entries[0].Amount=%d", entries[0].Amount)
	}
	if entries[1].Amount != 14 {
		t.Errorf("entries[1].Amount=%d", entries[1].Amount)
	}
	if entries[1].TraceNumber != "124" {
		t.Errorf("entries[1].TraceNumber=%s", entries[1].TraceNumber)
	}
}

func TestMicroDeposits_submitMicroDeposits(t *testing.T) {
	w := httptest.NewRecorder()

	achClient, _, achServer := achclient.MockClientServer("submitMicroDeposits", func(r *mux.Router) {
		achclient.AddCreateRoute(w, r)
		achclient.AddValidateRoute(r)
	})
	defer achServer.Close()

	testODFIAccount := makeTestODFIAccount()

	router := &DepositoryRouter{
		logger:      log.NewNopLogger(),
		config:      config.Empty(),
		achClient:   achClient,
		eventRepo:   &testEventRepository{},
		odfiAccount: testODFIAccount,
	}
	router.accountsClient = nil // explicitly disable Accounts calls for this test
	userID, requestID := base.ID(), base.ID()

	amounts := []Amount{
		{symbol: "USD", number: 12}, // $0.12
		{symbol: "USD", number: 37}, // $0.37
	}

	dep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "121042882",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
	}

	microDeposits, err := router.submitMicroDeposits(userID, requestID, amounts, dep)
	if n := len(microDeposits); n != 2 || err != nil {
		t.Fatalf("n=%d error=%v", n, err)
	}
}

func TestMicroDeposits__LookupMicroDepositFromReturn(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLDepositoryRepo) {
		amt1, _ := NewAmount("USD", "0.11")
		amt2, _ := NewAmount("USD", "0.12")

		userID := base.ID()
		depID1, depID2 := DepositoryID(base.ID()), DepositoryID(base.ID())

		// initial lookups with no rows written
		if md, err := repo.LookupMicroDepositFromReturn(depID1, amt1); md != nil || err != nil {
			t.Errorf("micro-deposit=%#v error=%v", md, err)
		}
		if md, err := repo.LookupMicroDepositFromReturn(depID1, amt2); md != nil || err != nil {
			t.Errorf("micro-deposit=%#v error=%v", md, err)
		}
		if md, err := repo.LookupMicroDepositFromReturn(depID2, amt1); md != nil || err != nil {
			t.Errorf("micro-deposit=%#v error=%v", md, err)
		}
		if md, err := repo.LookupMicroDepositFromReturn(depID2, amt2); md != nil || err != nil {
			t.Errorf("micro-deposit=%#v error=%v", md, err)
		}

		// write a micro-deposit and then lookup
		microDeposits := []*MicroDeposit{
			{Amount: *amt1, FileID: "fileID", TransactionID: "transactionID"},
			{Amount: *amt2, FileID: "fileID2", TransactionID: "transactionID2"},
		}
		if err := repo.InitiateMicroDeposits(depID1, userID, microDeposits); err != nil {
			t.Fatal(err)
		}

		// lookups (matching cases)
		if md, err := repo.LookupMicroDepositFromReturn(depID1, amt1); md == nil || err != nil {
			t.Errorf("micro-deposit=%#v error=%v", md, err)
		}
		if md, err := repo.LookupMicroDepositFromReturn(depID1, amt2); md == nil || err != nil {
			t.Errorf("micro-deposit=%#v error=%v", md, err)
		}

		// lookups (not matching cases)
		if md, err := repo.LookupMicroDepositFromReturn(depID2, amt1); md != nil || err != nil {
			t.Errorf("micro-deposit=%#v error=%v", md, err)
		}
		if md, err := repo.LookupMicroDepositFromReturn(depID2, amt2); md != nil || err != nil {
			t.Errorf("micro-deposit=%#v error=%v", md, err)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLDepositoryRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLDepositoryRepo{mysqlDB.DB, log.NewNopLogger()})
}

func getReturnCode(t *testing.T, db *sql.DB, depID DepositoryID, amt *Amount) string {
	t.Helper()

	query := `select return_code from micro_deposits where depository_id = ? and amount = ? and deleted_at is null`
	stmt, err := db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	var returnCode string
	if err := stmt.QueryRow(depID, amt.String()).Scan(&returnCode); err != nil {
		if err == sql.ErrNoRows {
			return ""
		}
		t.Fatal(err)
	}
	return returnCode
}

func TestMicroDeposits__SetReturnCode(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLDepositoryRepo) {
		amt, _ := NewAmount("USD", "0.11")
		depID, userID := DepositoryID(base.ID()), base.ID()

		dep := &Depository{
			ID:     depID,
			Status: DepositoryRejected, // needs to be rejected for getMicroDepositReturnCodes
		}
		if err := repo.UpsertUserDepository(userID, dep); err != nil {
			t.Fatal(err)
		}

		// get an empty return_code as we've written nothing
		if code := getReturnCode(t, repo.db, depID, amt); code != "" {
			t.Fatalf("code=%s", code)
		}

		// write a micro-deposit and set the return code
		microDeposits := []*MicroDeposit{
			{Amount: *amt, FileID: "fileID", TransactionID: "transactionID"},
		}
		if err := repo.InitiateMicroDeposits(depID, userID, microDeposits); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetReturnCode(depID, *amt, "R14"); err != nil {
			t.Fatal(err)
		}

		// lookup again and expect the return_code
		if code := getReturnCode(t, repo.db, depID, amt); code != "R14" {
			t.Errorf("code=%s", code)
		}

		xs, err := repo.getMicroDepositsForUser(depID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if len(xs) == 0 {
			t.Error("no micro-deposits found")
		}

		// lookup with our SQLDepositoryRepo method
		codes := repo.getMicroDepositReturnCodes(depID)
		if len(codes) != 1 {
			t.Fatalf("got %d codes", len(codes))
		}
		if codes[0].Code != "R14" {
			t.Errorf("codes[0].Code=%s", codes[0].Code)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLDepositoryRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLDepositoryRepo{mysqlDB.DB, log.NewNopLogger()})
}

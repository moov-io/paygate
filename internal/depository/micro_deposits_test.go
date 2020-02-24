// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	moovaccounts "github.com/moov-io/accounts/client"
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/achclient"
	"github.com/moov-io/paygate/pkg/id"

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
	keeper := secrets.TestStringKeeper(t)
	accountsClient := &accounts.MockClient{}

	num, _ := keeper.EncryptString("1234")
	odfi := &ODFIAccount{
		client:        accountsClient,
		accountNumber: num,
		routingNumber: "",
		accountType:   model.Savings,
		accountID:     "accountID",
		keeper:        keeper,
	}

	orig, dep := odfi.metadata()
	if orig == nil || dep == nil {
		t.Fatalf("\norig=%#v\ndep=%#v", orig, dep)
	}
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
	accountsClient.Accounts = []moovaccounts.Account{
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
	accountsClient.Err = errors.New("bad")
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
	amt, _ := model.NewAmount("USD", "1.24")
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
		guesses       []model.Amount
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
				guesses:       []model.Amount{},
			},
			"unable to confirm micro deposits, got 0 micro deposits",
		},
		{
			"There are less guesses than microdeposits",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "10"))},
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "4"))},
				},
				guesses: []model.Amount{
					*model.MustAmount(t)(model.NewAmount("USD", "10")),
				},
			},
			"incorrect amount of guesses, got 1",
		},
		{
			"There are more guesses than microdeposits",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "10"))},
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "4"))},
				},
				guesses: []model.Amount{
					*model.MustAmount(t)(model.NewAmount("USD", "10")),
					*model.MustAmount(t)(model.NewAmount("USD", "7")),
					*model.MustAmount(t)(model.NewAmount("USD", "4")),
				},
			},
			"incorrect amount of guesses, got 3",
		},
		{
			"One guess is correct, the other is wrong",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "10"))},
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "4"))},
				},
				guesses: []model.Amount{
					*model.MustAmount(t)(model.NewAmount("USD", "10")),
					*model.MustAmount(t)(model.NewAmount("USD", "7")),
				},
			},
			"incorrect micro deposit guesses",
		},
		{
			"Both guesses are wrong",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "10"))},
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "4"))},
				},
				guesses: []model.Amount{
					*model.MustAmount(t)(model.NewAmount("USD", "1")),
					*model.MustAmount(t)(model.NewAmount("USD", "7")),
				},
			},
			"incorrect micro deposit guesses",
		},
		{
			"Both guesses are correct",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "10"))},
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "4"))},
				},
				guesses: []model.Amount{
					*model.MustAmount(t)(model.NewAmount("USD", "10")),
					*model.MustAmount(t)(model.NewAmount("USD", "4")),
				},
			},
			"",
		},
		{
			"Both guesses are correct, in the opposite order",
			state{
				microDeposits: []*MicroDeposit{
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "10"))},
					{Amount: *model.MustAmount(t)(model.NewAmount("USD", "4"))},
				},
				guesses: []model.Amount{
					*model.MustAmount(t)(model.NewAmount("USD", "4")),
					*model.MustAmount(t)(model.NewAmount("USD", "10")),
				},
			},
			"",
		},
	}

	keeper := secrets.TestStringKeeper(t)

	sqlite := database.CreateTestSqliteDB(t)
	defer sqlite.Close()
	mysql := database.CreateTestMySQLDB(t)
	defer mysql.Close()
	databases := []*SQLRepo{
		NewDepositoryRepo(log.NewNopLogger(), sqlite.DB, keeper),
		NewDepositoryRepo(log.NewNopLogger(), mysql.DB, keeper),
	}

	for _, db := range databases {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				depositoryID := id.Depository(base.ID())
				userID := id.User(base.ID())

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

	check := func(t *testing.T, repo Repository) {
		id, userID := id.Depository(base.ID()), id.User(base.ID())

		amt, _ := model.NewAmount("USD", "0.11")
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

	keeper := secrets.TestStringKeeper(t)

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), mysqlDB.DB, keeper))
}

func TestMicroDeposits__initiateError(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	id, userID := id.Depository(base.ID()), base.ID()
	depRepo := &MockRepository{Err: errors.New("bad error")}
	router := &Router{
		logger:               log.NewNopLogger(),
		depositoryRepo:       depRepo,
		microDepositAttemper: NewAttemper(log.NewNopLogger(), db.DB, 5),
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

func TestMicroDeposits__initiateNoAttemptsLeft(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	depID, userID := id.Depository(base.ID()), base.ID()
	depRepo := &MockRepository{
		Depositories: []*model.Depository{
			{
				ID:                     id.Depository(base.ID()),
				BankName:               "bank name",
				Holder:                 "holder",
				HolderType:             model.Individual,
				Type:                   model.Checking,
				RoutingNumber:          "121042882",
				EncryptedAccountNumber: "151",
				Status:                 model.DepositoryUnverified,
			},
		},
	}
	router := &Router{
		logger:               log.NewNopLogger(),
		depositoryRepo:       depRepo,
		microDepositAttemper: &testAttempter{err: errors.New("bad error")},
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits", depID), nil)
	req.Header.Set("x-user-id", userID)
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status %d: %s", w.Code, w.Body.String())
	}
}

func TestMicroDeposits__confirmError(t *testing.T) {
	id, userID := id.Depository(base.ID()), base.ID()
	depRepo := &MockRepository{Err: errors.New("bad error")}
	router := &Router{
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

func TestMicroDeposits__confirmAttempts(t *testing.T) {
	depID, userID := id.Depository(base.ID()), base.ID()
	depRepo := &MockRepository{
		Depositories: []*model.Depository{
			{
				ID:                     depID,
				BankName:               "bank name",
				Holder:                 "hashholder",
				HolderType:             model.Individual,
				Type:                   model.Checking,
				RoutingNumber:          "121042882",
				EncryptedAccountNumber: "151",
				Status:                 model.DepositoryUnverified,
			},
		},
	}
	router := &Router{
		logger:         log.NewNopLogger(),
		depositoryRepo: depRepo,
		microDepositAttemper: &testAttempter{
			available: false,
		},
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(confirmDepositoryRequest{
		Amounts: []string{"USD 0.11", "USD 0.12"}, // doesn't matter as we block
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits/confirm", depID), &buf)
	req.Header.Set("x-user-id", userID)
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status %d: %s", w.Code, w.Body.String())
	}
	if v := w.Body.String(); !strings.Contains(v, "no micro-deposit attempts available") {
		t.Errorf("unexpected error: %v", v)
	}
}

func TestMicroDeposits__stringifyAmounts(t *testing.T) {
	out := stringifyAmounts(nil)
	if out != "" {
		t.Errorf("got %s", out)
	}

	out = stringifyAmounts([]model.Amount{
		*model.MustAmount(t)(model.NewAmount("USD", "12")), // $0.12
	})
	if out != "USD 0.12" {
		t.Errorf("got %s", out)
	}

	out = stringifyAmounts([]model.Amount{
		*model.MustAmount(t)(model.NewAmount("USD", "12")), // $0.12
		*model.MustAmount(t)(model.NewAmount("USD", "34")), // $0.34
	})
	if out != "USD 0.12,USD 0.34" {
		t.Errorf("got %s", out)
	}
}

func TestMicroDeposits__routes(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, db *sql.DB, keeper *secrets.StringKeeper) {
		id, userID := id.Depository(base.ID()), id.User(base.ID())

		depRepo := NewDepositoryRepo(log.NewNopLogger(), db, keeper)
		eventRepo := events.NewRepo(log.NewNopLogger(), db)

		// Write depository
		num, _ := keeper.EncryptString("151")
		dep := &model.Depository{
			ID:                     id,
			BankName:               "bank name",
			Holder:                 "hashholder",
			HolderType:             model.Individual,
			Type:                   model.Checking,
			RoutingNumber:          "121042882",
			EncryptedAccountNumber: num,
			Status:                 model.DepositoryUnverified, // status is checked in InitiateMicroDeposits
			Created:                base.NewTime(time.Now().Add(-1 * time.Second)),
			Keeper:                 keeper,
		}
		if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
			t.Fatal(err)
		}

		accountID := base.ID()
		accountsClient := &accounts.MockClient{
			Accounts: []moovaccounts.Account{{ID: accountID}},
			Transaction: &moovaccounts.Transaction{
				ID: base.ID(),
			},
		}
		fedClient := &fed.TestClient{}

		achClient, _, server := achclient.MockClientServer("micro-deposits", func(r *mux.Router) {
			achclient.AddCreateRoute(nil, r)
			achclient.AddValidateRoute(r)
		})
		defer server.Close()

		testODFIAccount := makeTestODFIAccount()
		testODFIAccount.keeper = keeper

		router := &Router{
			logger:               log.NewNopLogger(),
			odfiAccount:          testODFIAccount,
			accountsClient:       accountsClient,
			achClient:            achClient,
			fedClient:            fedClient,
			depositoryRepo:       depRepo,
			eventRepo:            eventRepo,
			microDepositAttemper: NewAttemper(log.NewNopLogger(), db, 5),
			keeper:               keeper,
		}
		r := mux.NewRouter()
		router.RegisterRoutes(r)

		// inititate our micro deposits
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits", id), nil)
		req.Header.Set("x-user-id", userID.String())
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
		for i := range accountsClient.PostedTransactions {
			for j := range accountsClient.PostedTransactions[i].Lines {
				// Only take the credit amounts (as we only need the amount from one side of the dual entry)
				line := accountsClient.PostedTransactions[i].Lines[j]

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
		req.Header.Set("x-user-id", userID.String())
		r.ServeHTTP(w, req)
		w.Flush()

		if w.Code != http.StatusOK {
			t.Errorf("confirm got %d status: %v", w.Code, w.Body.String())
		}
	}

	keeper := secrets.TestStringKeeper(t)

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, sqliteDB.DB, keeper)

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, mysqlDB.DB, keeper)
}

func TestMicroDeposits__MarkMicroDepositAsMerged(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLRepo) {
		amt, _ := model.NewAmount("USD", "0.11")
		microDeposits := []*MicroDeposit{
			{Amount: *amt, FileID: "fileID"},
		}
		if err := repo.InitiateMicroDeposits(id.Depository("id"), "userID", microDeposits); err != nil {
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
		mergedFilename, err := ReadMergedFilename(repo, amt, id.Depository(mc.DepositoryID))
		if err != nil {
			t.Fatal(err)
		}
		if mergedFilename != "filename" {
			t.Errorf("mergedFilename=%s", mergedFilename)
		}
	}

	keeper := secrets.TestStringKeeper(t)

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), mysqlDB.DB, keeper))
}

func TestMicroDeposits__addMicroDeposit(t *testing.T) {
	amt, _ := model.NewAmount("USD", "0.28")

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

	withdrawAmount, _ := model.NewAmount("USD", "0.14") // not $0.12 on purpose

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

	keeper := secrets.TestStringKeeper(t)

	testODFIAccount := makeTestODFIAccount()
	testODFIAccount.keeper = keeper

	router := &Router{
		logger:      log.NewNopLogger(),
		achClient:   achClient,
		eventRepo:   &events.TestRepository{},
		odfiAccount: testODFIAccount,
		keeper:      keeper,
	}
	router.accountsClient = nil // explicitly disable Accounts calls for this test
	userID, requestID := id.User(base.ID()), base.ID()

	amounts := []model.Amount{
		*model.MustAmount(t)(model.NewAmount("USD", "12")), // $0.12
		*model.MustAmount(t)(model.NewAmount("USD", "37")), // $0.37
	}

	num, _ := keeper.EncryptString("151")
	dep := &model.Depository{
		ID:                     id.Depository(base.ID()),
		BankName:               "bank name",
		Holder:                 "hashholder",
		HolderType:             model.Individual,
		Type:                   model.Checking,
		RoutingNumber:          "121042882",
		EncryptedAccountNumber: num,
		Status:                 model.DepositoryUnverified,
		Keeper:                 keeper,
	}

	microDeposits, err := router.submitMicroDeposits(userID, requestID, amounts, dep)
	if n := len(microDeposits); n != 2 || err != nil {
		t.Fatalf("n=%d error=%v", n, err)
	}
}

func TestMicroDeposits__submitNoAttemptsLeft(t *testing.T) {
	testODFIAccount := makeTestODFIAccount()

	router := &Router{
		logger:               log.NewNopLogger(),
		odfiAccount:          testODFIAccount,
		microDepositAttemper: &testAttempter{err: errors.New("bad error")},
	}
	userID, requestID := id.User(base.ID()), base.ID()
	amounts := []model.Amount{
		*model.MustAmount(t)(model.NewAmount("USD", "12")), // $0.12
		*model.MustAmount(t)(model.NewAmount("USD", "37")), // $0.37
	}
	dep := &model.Depository{
		ID:                     id.Depository(base.ID()),
		BankName:               "bank name",
		Holder:                 "hashholder",
		HolderType:             model.Individual,
		Type:                   model.Checking,
		RoutingNumber:          "121042882",
		EncryptedAccountNumber: "151",
		Status:                 model.DepositoryUnverified,
	}
	microDeposits, err := router.submitMicroDeposits(userID, requestID, amounts, dep)
	if len(microDeposits) != 0 || err == nil {
		t.Errorf("expected error: microDeposits=%#v", microDeposits)
	}
}

func TestMicroDeposits__LookupMicroDepositFromReturn(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLRepo) {
		amt1, _ := model.NewAmount("USD", "0.11")
		amt2, _ := model.NewAmount("USD", "0.12")

		userID := id.User(base.ID())
		depID1, depID2 := id.Depository(base.ID()), id.Depository(base.ID())

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

	keeper := secrets.TestStringKeeper(t)

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), mysqlDB.DB, keeper))
}

func getReturnCode(t *testing.T, db *sql.DB, depID id.Depository, amt *model.Amount) string {
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

	check := func(t *testing.T, repo *SQLRepo) {
		amt, _ := model.NewAmount("USD", "0.11")
		depID, userID := id.Depository(base.ID()), id.User(base.ID())

		dep := &model.Depository{
			ID:     depID,
			Status: model.DepositoryRejected, // needs to be rejected for getMicroDepositReturnCodes
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

		// lookup with our SQLRepo method
		codes := repo.getMicroDepositReturnCodes(depID)
		if len(codes) != 1 {
			t.Fatalf("got %d codes", len(codes))
		}
		if codes[0].Code != "R14" {
			t.Errorf("codes[0].Code=%s", codes[0].Code)
		}
	}

	keeper := secrets.TestStringKeeper(t)

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), mysqlDB.DB, keeper))
}

func TestMicroDepositsHTTP__initiateNoUserID(t *testing.T) {
	repo := &MockRepository{}
	router := &Router{
		logger:         log.NewNopLogger(),
		depositoryRepo: repo,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/depositories/foo/micro-deposits", nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestMicroDepositsHTTP__confirmNoUserID(t *testing.T) {
	repo := &MockRepository{}
	router := &Router{
		logger:         log.NewNopLogger(),
		depositoryRepo: repo,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/depositories/foo/micro-deposits/confirm", nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

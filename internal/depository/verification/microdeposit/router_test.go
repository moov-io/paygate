// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

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

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/gateways"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestMicroDeposits__json(t *testing.T) {
	amt, _ := model.NewAmount("USD", "1.24")
	bs, err := json.Marshal([]Credit{
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
		guesses []model.Amount
		credits []*Credit
	}
	testCases := []struct {
		name               string
		state              state
		expectedErrMessage string
	}{
		{
			"There are 0 microdeposits",
			state{
				credits: []*Credit{},
				guesses: []model.Amount{},
			},
			"unable to confirm micro deposits, got 0 micro deposits",
		},
		{
			"There are less guesses than microdeposits",
			state{
				credits: []*Credit{
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
				credits: []*Credit{
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
				credits: []*Credit{
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
				credits: []*Credit{
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
				credits: []*Credit{
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
				credits: []*Credit{
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

	sqlite := database.CreateTestSqliteDB(t)
	defer sqlite.Close()

	mysql := database.CreateTestMySQLDB(t)
	defer mysql.Close()

	databases := []*SQLRepo{
		NewRepository(log.NewNopLogger(), sqlite.DB),
		NewRepository(log.NewNopLogger(), mysql.DB),
	}

	for _, db := range databases {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				depositoryID := id.Depository(base.ID())
				userID := id.User(base.ID())

				if err := db.InitiateMicroDeposits(depositoryID, userID, tc.state.credits); err != nil {
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
		mc := &Credit{
			Amount:        *amt,
			FileID:        base.ID() + "-micro-deposit-verify",
			TransactionID: "transactionID",
		}
		mcs := []*Credit{mc}

		if err := repo.InitiateMicroDeposits(id, userID, mcs); err != nil {
			t.Fatal(err)
		}

		microDeposits, err := repo.GetMicroDepositsForUser(id, userID)
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
	check(t, NewRepository(log.NewNopLogger(), sqliteDB.DB))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewRepository(log.NewNopLogger(), mysqlDB.DB))
}

func TestMicroDeposits__initiateError(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	keeper := secrets.TestStringKeeper(t)
	depRepo := depository.NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)

	id, userID := id.Depository(base.ID()), id.User(base.ID())
	dep := &model.Depository{
		ID: id,
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	repo := &MockRepository{Err: errors.New("bad error")}
	router := &Router{
		logger:         log.NewNopLogger(),
		repo:           repo,
		depositoryRepo: depRepo,
		attempter:      NewAttemper(log.NewNopLogger(), db.DB, 5),
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/depositories/%s/micro-deposits", id), nil)
	req.Header.Set("x-user-id", userID.String())
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
	depRepo := &depository.MockRepository{
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
		logger:         log.NewNopLogger(),
		depositoryRepo: depRepo,
		attempter:      &testAttempter{err: errors.New("bad error")},
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
	depRepo := &depository.MockRepository{Err: errors.New("bad error")}
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
	depRepo := &depository.MockRepository{
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
		attempter: &testAttempter{
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

		repo := &MockRepository{}
		depRepo := depository.NewDepositoryRepo(log.NewNopLogger(), db, keeper)
		eventRepo := events.NewRepo(log.NewNopLogger(), db)
		gatewayRepo := &gateways.MockRepository{
			Gateway: &model.Gateway{
				ID: model.GatewayID(base.ID()),
			},
		}

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
			Accounts: []accounts.Account{{ID: accountID}},
			Transaction: &accounts.Transaction{
				ID: base.ID(),
			},
		}

		testODFIAccount := makeTestODFIAccount()
		testODFIAccount.keeper = keeper

		router := NewRouter(
			log.NewNopLogger(),
			testODFIAccount,
			NewAttemper(log.NewNopLogger(), db, 5),
			accountsClient,
			depRepo,
			eventRepo,
			gatewayRepo,
			repo,
		)

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
				t.Errorf("initiate got %d status: %v", w.Code, w.Body.String())
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
		microDeposits := []*Credit{
			{Amount: *amt, FileID: "fileID"},
		}
		if err := repo.InitiateMicroDeposits(id.Depository("id"), "userID", microDeposits); err != nil {
			t.Fatal(err)
		}

		mc := UploadableCredit{
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

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewRepository(log.NewNopLogger(), sqliteDB.DB))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewRepository(log.NewNopLogger(), mysqlDB.DB))
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

func TestMicroDeposits__addMicroDepositDebit(t *testing.T) {
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

	debitAmount, _ := model.NewAmount("USD", "0.14") // not $0.12 on purpose

	// nil, so expect no changes
	if err := addMicroDepositDebit(nil, debitAmount); err == nil {
		t.Fatal("expected error")
	}
	if len(file.Batches) != 1 || len(file.Batches[0].GetEntries()) != 1 {
		t.Fatalf("file.Batches[0]=%#v", file.Batches[0])
	}

	// add reversal batch
	if err := addMicroDepositDebit(file, debitAmount); err != nil {
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
	keeper := secrets.TestStringKeeper(t)

	testODFIAccount := makeTestODFIAccount()
	testODFIAccount.keeper = keeper

	router := &Router{
		logger:    log.NewNopLogger(),
		eventRepo: &events.TestRepository{},
		gatewayRepo: &gateways.MockRepository{
			Gateway: &model.Gateway{
				ID: model.GatewayID(base.ID()),
			},
		},
		odfiAccount: testODFIAccount,
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
		logger:      log.NewNopLogger(),
		odfiAccount: testODFIAccount,
		attempter:   &testAttempter{err: errors.New("bad error")},
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
		credits := []*Credit{
			{Amount: *amt1, FileID: "fileID", TransactionID: "transactionID"},
			{Amount: *amt2, FileID: "fileID2", TransactionID: "transactionID2"},
		}
		if err := repo.InitiateMicroDeposits(depID1, userID, credits); err != nil {
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
	check(t, NewRepository(log.NewNopLogger(), sqliteDB.DB))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewRepository(log.NewNopLogger(), mysqlDB.DB))
}

func TestMicroDepositsHTTP__initiateNoUserID(t *testing.T) {
	repo := &MockRepository{}
	router := &Router{
		logger: log.NewNopLogger(),
		repo:   repo,
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
		logger: log.NewNopLogger(),
		repo:   repo,
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

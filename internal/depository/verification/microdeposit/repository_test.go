// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
	"database/sql"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

func TestDepositories__markApproved(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, depRepo depository.Repository) {
		userID := id.User(base.ID())
		dep := &model.Depository{
			ID:                     id.Depository(base.ID()),
			BankName:               "bank name",
			Holder:                 "holder",
			HolderType:             model.Individual,
			Type:                   model.Checking,
			RoutingNumber:          "123",
			EncryptedAccountNumber: "151",
			Status:                 model.DepositoryUnverified,
			Created:                base.NewTime(time.Now().Add(-1 * time.Second)),
		}
		if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
			t.Error(err)
		}

		// read
		d, err := depRepo.GetUserDepository(dep.ID, userID)
		if err != nil || d == nil {
			t.Errorf("expected depository, d=%v, err=%v", d, err)
		}
		if d.Status != model.DepositoryUnverified {
			t.Errorf("got %v", d.Status)
		}

		// Verify, then re-check
		if err := markDepositoryVerified(depRepo, dep.ID, userID); err != nil {
			t.Fatal(err)
		}

		d, err = depRepo.GetUserDepository(dep.ID, userID)
		if err != nil || d == nil {
			t.Errorf("expected depository, d=%v, err=%v", d, err)
		}
		if d.Status != model.DepositoryVerified {
			t.Errorf("got %v", d.Status)
		}
	}

	keeper := secrets.TestStringKeeper(t)

	// SQLite
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, depository.NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper))

	// MySQL
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, depository.NewDepositoryRepo(log.NewNopLogger(), mysqlDB.DB, keeper))
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

	check := func(t *testing.T, repo *SQLRepo, depRepo depository.Repository) {
		amt, _ := model.NewAmount("USD", "0.11")
		depID, userID := id.Depository(base.ID()), id.User(base.ID())

		dep := &model.Depository{
			ID:     depID,
			Status: model.DepositoryRejected, // needs to be rejected for getMicroDepositReturnCodes
		}
		if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
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
		if err := depRepo.SetReturnCode(depID, *amt, "R14"); err != nil {
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
	check(t, NewRepository(log.NewNopLogger(), sqliteDB.DB), depository.NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewRepository(log.NewNopLogger(), mysqlDB.DB), depository.NewDepositoryRepo(log.NewNopLogger(), mysqlDB.DB, keeper))
}

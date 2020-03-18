// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package verification

import (
	"database/sql"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/depository/verification/microdeposit"
	"github.com/moov-io/paygate/internal/depository/verification/microdeposit/returns"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

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

	check := func(t *testing.T, db *sql.DB, keeper *secrets.StringKeeper) {
		amt, _ := model.NewAmount("USD", "0.11")
		depID, userID := id.Depository(base.ID()), id.User(base.ID())

		depRepo := depository.NewDepositoryRepo(log.NewNopLogger(), db, keeper)
		microDepositRepo := microdeposit.NewRepository(log.NewNopLogger(), db)

		dep := &model.Depository{
			ID:     depID,
			Status: model.DepositoryRejected, // needs to be rejected for getMicroDepositReturnCodes
		}
		if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
			t.Fatal(err)
		}

		// get an empty return_code as we've written nothing
		if code := getReturnCode(t, db, depID, amt); code != "" {
			t.Fatalf("code=%s", code)
		}

		// write a micro-deposit and set the return code
		microDeposits := []*microdeposit.Credit{
			{Amount: *amt, FileID: "fileID", TransactionID: "transactionID"},
		}
		if err := microDepositRepo.InitiateMicroDeposits(depID, userID, microDeposits); err != nil {
			t.Fatal(err)
		}
		if err := microDepositRepo.SetReturnCode(depID, *amt, "R14"); err != nil {
			t.Fatal(err)
		}

		// lookup again and expect the return_code
		if code := getReturnCode(t, db, depID, amt); code != "R14" {
			t.Errorf("code=%s", code)
		}

		xs, err := microDepositRepo.GetMicroDepositsForUser(depID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if len(xs) == 0 {
			t.Error("no micro-deposits found")
		}

		// lookup with our SQLRepo method
		codes := returns.FromMicroDeposits(db, depID)
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
	check(t, sqliteDB.DB, keeper)

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, mysqlDB.DB, keeper)
}

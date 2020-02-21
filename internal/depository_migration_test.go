// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

func writeAccountNumber(t *testing.T, number string, depID id.Depository, db *sql.DB) {
	query := `update depositories set account_number = ?, account_number_encrypted = '', account_number_hashed = '' where depository_id = ?;`
	stmt, err := db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(number, depID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDepository__grabEncryptableDepositories(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLDepositoryRepo) {
		depID := id.Depository(base.ID())
		userID := id.User(base.ID())

		keeper := secrets.TestStringKeeper(t)

		dep := &model.Depository{
			ID:            depID,
			RoutingNumber: "987654320",
			Type:          model.Checking,
			BankName:      "bank name",
			Holder:        "holder",
			HolderType:    model.Individual,
			Status:        model.DepositoryUnverified,
			Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
		}
		if err := repo.UpsertUserDepository(userID, dep); err != nil {
			t.Fatal(err)
		}

		writeAccountNumber(t, "123456", depID, repo.db)

		if err := EncryptStoredAccountNumbers(log.NewNopLogger(), repo, keeper); err != nil {
			t.Fatal(err)
		}

		dep, err := repo.GetDepository(dep.ID)
		if err != nil {
			t.Fatal(err)
		}
		if num, err := keeper.DecryptString(dep.EncryptedAccountNumber); err != nil {
			t.Fatal(err)
		} else {
			if num != "123456" {
				t.Fatalf("account number: %s", num)
			}
		}
	}

	keeper := secrets.TestStringKeeper(t)

	// SQLite
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper))

	// MySQL
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), mysqlDB.DB, keeper))
}

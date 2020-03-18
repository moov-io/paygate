// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
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

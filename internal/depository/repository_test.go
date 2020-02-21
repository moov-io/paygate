// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestDepositories__emptyDB(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
		userID := id.User(base.ID())
		if err := repo.deleteUserDepository(id.Depository(base.ID()), userID); err != nil {
			t.Errorf("expected no error, but got %v", err)
		}

		// all depositories for a user
		deps, err := repo.GetUserDepositories(userID)
		if err != nil {
			t.Error(err)
		}
		if len(deps) != 0 {
			t.Errorf("expected empty, got %v", deps)
		}

		// specific Depository
		dep, err := repo.GetUserDepository(id.Depository(base.ID()), userID)
		if err != nil {
			t.Error(err)
		}
		if dep != nil {
			t.Errorf("expected empty, got %v", dep)
		}

		// depository check
		dep, err = repo.GetUserDepository(id.Depository(base.ID()), userID)
		if dep != nil {
			t.Errorf("dep=%#v expected no depository", dep)
		}
		if err != nil {
			t.Error(err)
		}

		dep, err = repo.GetDepository(id.Depository(base.ID()))
		if dep != nil || err != nil {
			t.Errorf("expected no depository: %#v: %v", dep, err)
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

func TestDepositories__upsert(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
		userID := id.User(base.ID())
		dep := &model.Depository{
			ID:                     id.Depository(base.ID()),
			BankName:               "bank name",
			Holder:                 "holder",
			HolderType:             model.Individual,
			Type:                   model.Checking,
			RoutingNumber:          "123",
			EncryptedAccountNumber: "151",
			Status:                 model.DepositoryVerified,
			Created:                base.NewTime(time.Now().Add(-1 * time.Second)),
		}
		if d, err := repo.GetUserDepository(dep.ID, userID); err != nil || d != nil {
			t.Errorf("expected empty, d=%v | err=%v", d, err)
		}

		// write, then verify
		if err := repo.UpsertUserDepository(userID, dep); err != nil {
			t.Error(err)
		}

		d, err := repo.GetUserDepository(dep.ID, userID)
		if err != nil {
			t.Error(err)
		}
		if d == nil {
			t.Fatal("expected Depository, got nil")
		}
		if d.ID != dep.ID {
			t.Errorf("d.ID=%q, dep.ID=%q", d.ID, dep.ID)
		}

		// get all for our user
		depositories, err := repo.GetUserDepositories(userID)
		if err != nil {
			t.Error(err)
		}
		if len(depositories) != 1 {
			t.Errorf("expected one, got %v", depositories)
		}
		if depositories[0].ID != dep.ID {
			t.Errorf("depositories[0].ID=%q, dep.ID=%q", depositories[0].ID, dep.ID)
		}

		// update, verify default depository changed
		bankName := "my new bank"
		dep.BankName = bankName
		if err := repo.UpsertUserDepository(userID, dep); err != nil {
			t.Error(err)
		}
		d, err = repo.GetUserDepository(dep.ID, userID)
		if err != nil {
			t.Error(err)
		}
		if dep.BankName != d.BankName {
			t.Errorf("got %q", d.BankName)
		}
		if d.Status != model.DepositoryVerified {
			t.Errorf("status: %s", d.Status)
		}

		dep, err = repo.GetUserDepository(dep.ID, userID)
		if dep == nil || err != nil {
			t.Errorf("DepositoryId should exist: %v", err)
		}
		dep, err = repo.GetDepository(dep.ID)
		if dep == nil || err != nil {
			t.Errorf("expected depository=%#v: %v", dep, err)
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

func TestDepositories__delete(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
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
		if d, err := repo.GetUserDepository(dep.ID, userID); err != nil || d != nil {
			t.Errorf("expected empty, d=%v | err=%v", d, err)
		}

		// write
		if err := repo.UpsertUserDepository(userID, dep); err != nil {
			t.Error(err)
		}

		// verify
		d, err := repo.GetUserDepository(dep.ID, userID)
		if err != nil || d == nil {
			t.Errorf("expected depository, d=%v, err=%v", d, err)
		}

		// delete
		if err := repo.deleteUserDepository(dep.ID, userID); err != nil {
			t.Error(err)
		}

		// verify tombstoned
		if d, err := repo.GetUserDepository(dep.ID, userID); err != nil || d != nil {
			t.Errorf("expected empty, d=%v | err=%v", d, err)
		}

		dep, err = repo.GetUserDepository(dep.ID, userID)
		if dep != nil || err != nil {
			t.Errorf("dep=%#v expected none: error=%v", dep, err)
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

func TestDepositories__UpdateDepositoryStatus(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
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

		// write
		if err := repo.UpsertUserDepository(userID, dep); err != nil {
			t.Error(err)
		}

		// upsert and read back
		if err := repo.UpdateDepositoryStatus(dep.ID, model.DepositoryVerified); err != nil {
			t.Fatal(err)
		}
		dep2, err := repo.GetUserDepository(dep.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if dep.ID != dep2.ID {
			t.Errorf("expected=%s got=%s", dep.ID, dep2.ID)
		}
		if dep2.Status != model.DepositoryVerified {
			t.Errorf("unknown status: %s", dep2.Status)
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

func TestDepositories__markApproved(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
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

		// write
		if err := repo.UpsertUserDepository(userID, dep); err != nil {
			t.Error(err)
		}

		// read
		d, err := repo.GetUserDepository(dep.ID, userID)
		if err != nil || d == nil {
			t.Errorf("expected depository, d=%v, err=%v", d, err)
		}
		if d.Status != model.DepositoryUnverified {
			t.Errorf("got %v", d.Status)
		}

		// Verify, then re-check
		if err := markDepositoryVerified(repo, dep.ID, userID); err != nil {
			t.Fatal(err)
		}

		d, err = repo.GetUserDepository(dep.ID, userID)
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
	check(t, NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper))

	// MySQL
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewDepositoryRepo(log.NewNopLogger(), mysqlDB.DB, keeper))
}

func TestDepositories__LookupDepositoryFromReturn(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLRepo) {
		userID := id.User(base.ID())
		routingNumber, accountNumber := "987654320", "152311"

		// lookup when nothing will be returned
		dep, err := repo.LookupDepositoryFromReturn(routingNumber, accountNumber)
		if dep != nil || err != nil {
			t.Fatalf("depository=%#v error=%v", dep, err)
		}

		depID := id.Depository(base.ID())
		dep = &model.Depository{
			ID:            depID,
			RoutingNumber: routingNumber,
			Type:          model.Checking,
			BankName:      "bank name",
			Holder:        "holder",
			HolderType:    model.Individual,
			Status:        model.DepositoryUnverified,
			Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
			Keeper:        repo.keeper,
		}
		if err := dep.ReplaceAccountNumber(accountNumber); err != nil {
			t.Fatal(err)
		}
		if err := repo.UpsertUserDepository(userID, dep); err != nil {
			t.Fatal(err)
		}

		// lookup again now after we wrote the Depository
		dep, err = repo.LookupDepositoryFromReturn(routingNumber, accountNumber)
		if dep == nil || err != nil {
			t.Fatalf("depository=%#v error=%v", dep, err)
		}
		if depID != dep.ID {
			t.Errorf("depID=%s dep.ID=%s", depID, dep.ID)
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

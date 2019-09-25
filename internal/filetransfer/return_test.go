// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"testing"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
)

// depositoryReturnCode writes two Depository objects into a database and then calls updateDepositoryFromReturnCode
// over the provided return code. The two Depository objects returned are re-read from the database after.
func depositoryReturnCode(t *testing.T, code string) (*internal.Depository, *internal.Depository) {
	t.Helper()

	logger := log.NewNopLogger()

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	repo := internal.NewDepositoryRepo(logger, sqliteDB.DB)

	userID := base.ID()
	origDep := &internal.Depository{
		ID:       internal.DepositoryID(base.ID()),
		BankName: "originator bank",
		Status:   internal.DepositoryVerified,
	}
	if err := repo.UpsertUserDepository(userID, origDep); err != nil {
		t.Fatal(err)
	}
	recDep := &internal.Depository{
		ID:       internal.DepositoryID(base.ID()),
		BankName: "receiver bank",
		Status:   internal.DepositoryVerified,
	}
	if err := repo.UpsertUserDepository(userID, recDep); err != nil {
		t.Fatal(err)
	}

	rc := &ach.ReturnCode{Code: code}
	if err := updateDepositoryFromReturnCode(logger, rc, origDep, recDep, repo); err != nil {
		t.Fatal(err)
	}

	// re-read and return the Depository objects
	oDep, _ := repo.GetUserDepository(origDep.ID, userID)
	rDep, _ := repo.GetUserDepository(recDep.ID, userID)
	return oDep, rDep
}

func TestDepositories__UpdateDepositoryFromReturnCode(t *testing.T) {
	cases := []struct {
		code                  string
		origStatus, recStatus internal.DepositoryStatus
	}{
		// R02, R07, R10
		{"R02", internal.DepositoryVerified, internal.DepositoryRejected},
		{"R07", internal.DepositoryVerified, internal.DepositoryRejected},
		{"R10", internal.DepositoryVerified, internal.DepositoryRejected},
		// R05
		{"R05", internal.DepositoryVerified, internal.DepositoryRejected},
		// R14, R15
		{"R14", internal.DepositoryRejected, internal.DepositoryRejected},
		{"R15", internal.DepositoryRejected, internal.DepositoryRejected},
		// R16
		{"R16", internal.DepositoryVerified, internal.DepositoryRejected},
		// R20
		{"R20", internal.DepositoryVerified, internal.DepositoryRejected},
	}
	for i := range cases {
		orig, rec := depositoryReturnCode(t, cases[i].code)
		if orig == nil || rec == nil {
			t.Fatalf("  orig=%#v\n  rec=%#v", orig, rec)
		}
		if orig.Status != cases[i].origStatus || rec.Status != cases[i].recStatus {
			t.Errorf("%s: orig.Status=%s rec.Status=%s", cases[i].code, orig.Status, rec.Status)
		}
	}
}

func setupReturnCodeDepository() *internal.Depository {
	return &internal.Depository{
		ID:            internal.DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    internal.Individual,
		Type:          internal.Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        internal.DepositoryUnverified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}
}

func TestFiles__UpdateDepositoryFromReturnCode(t *testing.T) {
	Orig, Rec := 1, 2 // enum for 'check(..)'
	logger := log.NewNopLogger()

	check := func(t *testing.T, code string, cond int) {
		t.Helper()

		db := database.CreateTestSqliteDB(t)
		defer db.Close()

		userID := base.ID()
		repo := internal.NewDepositoryRepo(log.NewNopLogger(), db.DB)

		// Setup depositories
		origDep, receiverDep := setupReturnCodeDepository(), setupReturnCodeDepository()
		repo.UpsertUserDepository(userID, origDep)
		repo.UpsertUserDepository(userID, receiverDep)

		// after writing Depositories call updateDepositoryFromReturnCode
		if err := updateDepositoryFromReturnCode(logger, &ach.ReturnCode{Code: code}, origDep, receiverDep, repo); err != nil {
			t.Error(err)
		}
		var dep *internal.Depository
		if cond == Orig {
			dep, _ = repo.GetUserDepository(origDep.ID, userID)
			if dep.ID != origDep.ID {
				t.Error("read wrong Depository")
			}
		} else {
			dep, _ = repo.GetUserDepository(receiverDep.ID, userID)
			if dep.ID != receiverDep.ID {
				t.Error("read wrong Depository")
			}
		}
		if dep.Status != internal.DepositoryRejected {
			t.Errorf("unexpected status: %s", dep.Status)
		}
	}

	// Our testcases
	check(t, "R02", Rec)
	check(t, "R07", Rec)
	check(t, "R10", Rec)
	check(t, "R14", Orig)
	check(t, "R15", Orig)
	check(t, "R16", Rec)
	check(t, "R20", Rec)
}

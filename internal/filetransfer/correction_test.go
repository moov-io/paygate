// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
)

// depositoryChangeCode writes a Depository and then calls updateDepositoryFromChangeCode given the provided change code.
// The Depository is then re-read and returned from this method
func depositoryChangeCode(t *testing.T, changeCode string) *internal.Depository {
	logger := log.NewNopLogger()

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	repo := internal.NewDepositoryRepo(logger, sqliteDB.DB)

	userID := base.ID()
	dep := &internal.Depository{
		ID:       internal.DepositoryID(base.ID()),
		BankName: "my bank",
		Status:   internal.DepositoryVerified,
	}
	if err := repo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	ed := &ach.EntryDetail{
		Addenda98: &ach.Addenda98{
			CorrectedData: "", // make it non-nil
		},
	}
	cc := &ach.ChangeCode{Code: changeCode}

	if err := updateDepositoryFromChangeCode(logger, cc, ed, dep, repo); err != nil {
		t.Fatal(err)
	}

	dep, _ = repo.GetUserDepository(dep.ID, userID)
	return dep
}

func TestDepositories__updateDepositoryFromChangeCode(t *testing.T) {
	cases := []struct {
		code     string
		expected internal.DepositoryStatus
	}{
		// First Section
		{"C01", internal.DepositoryRejected},
		{"C02", internal.DepositoryRejected},
		{"C03", internal.DepositoryRejected},
		{"C04", internal.DepositoryRejected},
		{"C06", internal.DepositoryRejected},
		{"C07", internal.DepositoryRejected},
		{"C09", internal.DepositoryRejected},
		// Second Section
		{"C08", internal.DepositoryRejected},
		// Third Section // TODO(adam): these are unimplemented right now
		// {"C05", internal.DepositoryVerified},
		// {"C13", internal.DepositoryVerified},
		// {"C14", internal.DepositoryVerified},
	}
	for i := range cases {
		dep := depositoryChangeCode(t, cases[i].code)
		if dep == nil {
			t.Fatal("nil Depository")
		}
		if dep.Status != cases[i].expected {
			t.Errorf("%s: dep.Status=%v", cases[i].code, dep.Status)
		}
	}
}

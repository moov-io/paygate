// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestMicroDepositCursor__next(t *testing.T) {
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	keeper := secrets.TestStringKeeper(t)

	depRepo := NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper)
	cur := depRepo.GetMicroDepositCursor(2)

	microDeposits, err := cur.Next()
	if len(microDeposits) != 0 || err != nil {
		t.Fatalf("microDeposits=%#v error=%v", microDeposits, err)
	}

	// Write a micro-deposit
	amt, _ := model.NewAmount("USD", "0.11")
	if err := depRepo.InitiateMicroDeposits(id.Depository("id"), "userID", []*MicroDeposit{{Amount: *amt, FileID: "fileID"}}); err != nil {
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
	filename, err := ReadMergedFilename(depRepo, mc.Amount, id.Depository(mc.DepositoryID))
	if err != nil {
		t.Fatal(err)
	}
	if filename != "filename" {
		t.Errorf("mc=%#v", mc)
	}
}

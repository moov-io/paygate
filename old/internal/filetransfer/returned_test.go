// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"testing"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

// depositoryReturnCode writes two Depository objects into a database and then calls updateDepositoryFromReturnCode
// over the provided return code. The two Depository objects returned are re-read from the database after.
func depositoryReturnCode(t *testing.T, code string) (*model.Depository, *model.Depository) {
	logger := log.NewNopLogger()

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	keeper := secrets.TestStringKeeper(t)
	repo := depository.NewDepositoryRepo(logger, sqliteDB.DB, keeper)

	userID := id.User(base.ID())
	origDep := &model.Depository{
		ID:       id.Depository(base.ID()),
		BankName: "originator bank",
		Status:   model.DepositoryVerified,
	}
	if err := repo.UpsertUserDepository(userID, origDep); err != nil {
		t.Fatal(err)
	}
	recDep := &model.Depository{
		ID:       id.Depository(base.ID()),
		BankName: "receiver bank",
		Status:   model.DepositoryVerified,
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
		origStatus, recStatus model.DepositoryStatus
	}{
		{"R02", model.DepositoryVerified, model.DepositoryRejected}, // Account Closed
		{"R03", model.DepositoryVerified, model.DepositoryRejected}, // No Account/Unable to Locate Account
		{"R04", model.DepositoryVerified, model.DepositoryRejected}, // Invalid Account Number Structure
		{"R05", model.DepositoryVerified, model.DepositoryRejected}, // Improper Debit to Consumer Account
		{"R07", model.DepositoryVerified, model.DepositoryRejected}, // Authorization Revoked by Customer
		{"R10", model.DepositoryVerified, model.DepositoryRejected}, // Customer Advises Not Authorized
		{"R12", model.DepositoryVerified, model.DepositoryRejected}, // Account Sold to Another DFI
		{"R13", model.DepositoryVerified, model.DepositoryRejected}, // Invalid ACH Routing Number
		{"R16", model.DepositoryVerified, model.DepositoryRejected}, // Account Frozen/Entry Returned per OFAC Instruction
		{"R20", model.DepositoryVerified, model.DepositoryRejected}, // Non-payment (or non-transaction) bank account
		{"R28", model.DepositoryVerified, model.DepositoryRejected}, // Routing Number Check Digit Error
		{"R29", model.DepositoryVerified, model.DepositoryRejected}, // Corporate Customer Advises Not Authorized
		{"R30", model.DepositoryVerified, model.DepositoryRejected}, // RDFI Not Participant in Check Truncation Program
		{"R32", model.DepositoryVerified, model.DepositoryRejected}, // RDFI Non-Settlement
		{"R34", model.DepositoryVerified, model.DepositoryRejected}, // Limited Participation DFI
		{"R37", model.DepositoryVerified, model.DepositoryRejected}, // Source Document Presented for Payment
		{"R38", model.DepositoryVerified, model.DepositoryRejected}, // Stop Payment on Source Document
		{"R39", model.DepositoryVerified, model.DepositoryRejected}, // Improper Source Document/Source Document Presented for Payment
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

	// Return codes which don't Reject the Depository
	codes := []string{
		"R01", // Insufficient Funds
		"R06", // Returned per ODFI's Request
		"R08", // Payment Stopped
		"R09", // Uncollected Funds
		"R11", // Check Truncation Early Return
		"R17", // File Record Edit Criteria
		"R18", // Improper Effective Entry Date
		"R19", // Amount Field Error
		"R21", // Invalid Company Identification
		"R22", // Invalid Individual ID Number
		"R23", // Credit Entry Refused by Receiver
		"R24", // Duplicate Entry
		"R25", // Addenda Error
		"R26", // Mandatory Field Error
		"R27", // Trace Number Error
		"R31", // Permissible Return Entry (CCD and CTX Only)
		"R33", // Return of XCK Entry
		"R35", // Return of Improper Debit Entry
		"R36", // Return of Improper Credit Entry
	}
	for i := range codes {
		orig, rec := depositoryReturnCode(t, codes[i])
		if orig == nil || rec == nil {
			t.Fatalf("  orig=%#v\n  rec=%#v", orig, rec)
		}
		if orig.Status != model.DepositoryVerified {
			t.Fatalf("orig.Status=%s", orig.Status)
		}
		if rec.Status != model.DepositoryVerified {
			t.Fatalf("rec.Status=%s", rec.Status)
		}
	}
}

func setupReturnCodeDepository() *model.Depository {
	return &model.Depository{
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
}

func TestFiles__UpdateDepositoryFromReturnCode(t *testing.T) {
	Orig, Rec := 1, 2 // enum for 'check(..)'
	logger := log.NewNopLogger()

	check := func(t *testing.T, code string, cond int) {
		t.Helper()

		db := database.CreateTestSqliteDB(t)
		defer db.Close()

		userID := id.User(base.ID())
		keeper := secrets.TestStringKeeper(t)
		repo := depository.NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)

		// Setup depositories
		origDep, receiverDep := setupReturnCodeDepository(), setupReturnCodeDepository()
		repo.UpsertUserDepository(userID, origDep)
		repo.UpsertUserDepository(userID, receiverDep)

		// after writing Depositories call updateDepositoryFromReturnCode
		if err := updateDepositoryFromReturnCode(logger, &ach.ReturnCode{Code: code}, origDep, receiverDep, repo); err != nil {
			t.Error(err)
		}
		var dep *model.Depository
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
		if dep.Status != model.DepositoryRejected {
			t.Errorf("unexpected status: %s", dep.Status)
		}
	}

	// Codes which update Originator Depository
	check(t, "R14", Orig)
	check(t, "R15", Orig)

	// Codes which update Receiver Depository
	check(t, "R02", Rec) // Account Closed
	check(t, "R03", Rec) // No Account/Unable to Locate Account
	check(t, "R04", Rec) // Invalid Account Number Structure
	check(t, "R05", Rec) // Improper Debit to Consumer Account
	check(t, "R07", Rec) // Authorization Revoked by Customer
	check(t, "R10", Rec) // Customer Advises Not Authorized
	check(t, "R12", Rec) // Account Sold to Another DFI
	check(t, "R13", Rec) // Invalid ACH Routing Number
	check(t, "R16", Rec) // Account Frozen/Entry Returned per OFAC Instruction
	check(t, "R20", Rec) // Non-payment (or non-transaction) bank account
	check(t, "R28", Rec) // Routing Number Check Digit Error
	check(t, "R29", Rec) // Corporate Customer Advises Not Authorized
	check(t, "R30", Rec) // RDFI Not Participant in Check Truncation Program
	check(t, "R32", Rec) // RDFI Non-Settlement
	check(t, "R34", Rec) // Limited Participation DFI
	check(t, "R37", Rec) // Source Document Presented for Payment
	check(t, "R38", Rec) // Stop Payment on Source Document
	check(t, "R39", Rec) // Improper Source Document/Source Document Presented for Payment
}

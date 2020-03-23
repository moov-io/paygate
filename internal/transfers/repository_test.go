// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"database/sql"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

func TestTransfers__UpdateTransferStatus(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
		amt, _ := model.NewAmount("USD", "32.92")
		userID := id.User(base.ID())
		req := &transferRequest{
			Type:                   model.PushTransfer,
			Amount:                 *amt,
			Originator:             model.OriginatorID("originator"),
			OriginatorDepository:   id.Depository("originator"),
			Receiver:               model.ReceiverID("receiver"),
			ReceiverDepository:     id.Depository("receiver"),
			Description:            "money",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file",
		}
		transfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.UpdateTransferStatus(transfers[0].ID, model.TransferReclaimed); err != nil {
			t.Fatal(err)
		}

		xfer, err := repo.getUserTransfer(transfers[0].ID, userID)
		if err != nil {
			t.Error(err)
		}
		if xfer.Status != model.TransferReclaimed {
			t.Errorf("got status %s", xfer.Status)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestTransfers__transactionID(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, db *sql.DB) {
		userID := id.User(base.ID())
		transactionID := base.ID() // field we care about
		amt, _ := model.NewAmount("USD", "51.21")

		repo := &SQLRepo{db, log.NewNopLogger()}
		requests := []*transferRequest{
			{
				Type:                   model.PullTransfer,
				Amount:                 *amt,
				Originator:             model.OriginatorID("originator"),
				OriginatorDepository:   id.Depository("originatorDep"),
				Receiver:               model.ReceiverID("receiver"),
				ReceiverDepository:     id.Depository("receiverDep"),
				Description:            "money2",
				StandardEntryClassCode: "PPD",
				transactionID:          transactionID,
			},
		}
		if _, err := repo.createUserTransfers(userID, requests); err != nil {
			t.Fatal(err)
		}

		transfers, err := repo.getUserTransfers(userID)
		if err != nil || len(transfers) != 1 {
			t.Errorf("got %d Transfers (error=%v): %v", len(transfers), err, transfers)
		}

		query := `select transaction_id from transfers where transfer_id = ?`
		stmt, err := db.Prepare(query)
		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()

		var txID string
		row := stmt.QueryRow(transfers[0].ID)
		if err := row.Scan(&txID); err != nil {
			t.Fatal(err)
		}
		if txID != transactionID {
			t.Errorf("incorrect transactionID: %s vs %s", txID, transactionID)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, sqliteDB.DB)

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, mysqlDB.DB)
}

func TestTransfers__LookupTransferFromReturn(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
		amt, _ := model.NewAmount("USD", "32.92")
		userID := id.User(base.ID())
		req := &transferRequest{
			Type:                   model.PushTransfer,
			Amount:                 *amt,
			Originator:             model.OriginatorID("originator"),
			OriginatorDepository:   id.Depository("originator"),
			Receiver:               model.ReceiverID("receiver"),
			ReceiverDepository:     id.Depository("receiver"),
			Description:            "money",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file",
		}
		transfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
		if err != nil {
			t.Fatal(err)
		}

		// set metadata after transfer is merged into an ACH file for the FED
		if err := repo.MarkTransferAsMerged(transfers[0].ID, "merged.ach", "traceNumber"); err != nil {
			t.Fatal(err)
		}
		if err := repo.UpdateTransferStatus(transfers[0].ID, model.TransferProcessed); err != nil {
			t.Fatal(err)
		}

		// Now grab the transfer back
		xfer, err := repo.LookupTransferFromReturn("PPD", amt, "traceNumber", time.Now()) // EffectiveEntryDate is bounded by start and end of a day
		if err != nil {
			t.Fatal(err)
		}
		if xfer.ID != transfers[0].ID || xfer.UserID != userID.String() {
			t.Errorf("found other transfer=%q user=(%q vs %q)", xfer.ID, xfer.UserID, userID)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestTransfers__SetReturnCode(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, db *sql.DB) {
		userID := id.User(base.ID())
		returnCode := "R17"
		amt, _ := model.NewAmount("USD", "51.21")

		repo := &SQLRepo{db, log.NewNopLogger()}
		requests := []*transferRequest{
			{
				Type:                   model.PullTransfer,
				Amount:                 *amt,
				Originator:             model.OriginatorID("originator"),
				OriginatorDepository:   id.Depository("originatorDep"),
				Receiver:               model.ReceiverID("receiver"),
				ReceiverDepository:     id.Depository("receiverDep"),
				Description:            "money2",
				StandardEntryClassCode: "PPD",
			},
		}
		if _, err := repo.createUserTransfers(userID, requests); err != nil {
			t.Fatal(err)
		}

		transfers, err := repo.getUserTransfers(userID)
		if err != nil || len(transfers) != 1 {
			t.Errorf("got %d Transfers (error=%v): %v", len(transfers), err, transfers)
		}

		// Set ReturnCode
		if err := repo.SetReturnCode(transfers[0].ID, returnCode); err != nil {
			t.Fatal(err)
		}

		// Verify
		transfers, err = repo.getUserTransfers(userID)
		if err != nil || len(transfers) != 1 {
			t.Errorf("got %d Transfers (error=%v): %v", len(transfers), err, transfers)
		}
		if transfers[0].ReturnCode == nil {
			t.Fatal("expected ReturnCode")
		}
		if transfers[0].ReturnCode.Code != returnCode {
			t.Errorf("transfers[0].ReturnCode.Code=%s", transfers[0].ReturnCode.Code)
		}

		t.Logf("%#v", transfers[0].ReturnCode)
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, sqliteDB.DB)

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, mysqlDB.DB)
}

func TestTransfers__MarkTransfersAsProcessed(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLRepo) {
		amt, _ := model.NewAmount("USD", "32.92")
		userID := id.User(base.ID())
		req := &transferRequest{
			Type:                   model.PushTransfer,
			Amount:                 *amt,
			Originator:             model.OriginatorID("originator"),
			OriginatorDepository:   id.Depository("originator"),
			Receiver:               model.ReceiverID("receiver"),
			ReceiverDepository:     id.Depository("receiver"),
			Description:            "money",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file",
		}
		transfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
		if err != nil {
			t.Fatal(err)
		}
		if len(transfers) != 1 {
			t.Fatalf("got transfers=%#v", transfers)
		}

		filename := "20200320-0000001.ach"
		traceNumber := "000000987631"

		// merge Transfer
		if err := repo.MarkTransferAsMerged(transfers[0].ID, filename, traceNumber); err != nil {
			t.Fatal(err)
		}

		// prep Transfer for upload
		// include an extra traceNumber for sql query generator
		if n, err := repo.MarkTransfersAsProcessed(filename, []string{traceNumber, "00032"}); n != 1 || err != nil {
			t.Fatalf("n=%d error=%v", n, err)
		}

		// Check transfer status
		xfer, err := repo.getUserTransfer(transfers[0].ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if xfer.Status != model.TransferProcessed {
			t.Errorf("xfer.Status=%v", xfer.Status)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewTransferRepo(log.NewNopLogger(), sqliteDB.DB))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewTransferRepo(log.NewNopLogger(), mysqlDB.DB))
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/database"
)

func TestRepository__MarkMicroDepositsAsProcessed(t *testing.T) {
	t.Parallel()

	// Test where we update micro_deposits and transfers

	check := func(t *testing.T, repo *sqlRepo) {
		transferID, microDepositID := base.ID(), base.ID()

		writeTransfer(t, repo, transferID)
		writeMicroDeposit(t, repo, microDepositID, transferID)

		if err := repo.MarkTransfersAsProcessed([]string{transferID}); err != nil {
			t.Fatal(err)
		}

		status := getMicroDepositStatus(t, repo, microDepositID)
		if status != client.PROCESSED {
			t.Errorf("unexpected micro-deposit status: %s", status)
		}

		status = getTransferStatus(t, repo, transferID)
		if status != client.PROCESSED {
			t.Errorf("unexpected transfer status: %s", status)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

func TestRepository__MarkTransfersProcessed(t *testing.T) {
	t.Parallel()

	// Test where we only update transfers (no micro_deposits)

	check := func(t *testing.T, repo *sqlRepo) {
		transferID := base.ID()
		writeTransfer(t, repo, transferID)

		if err := repo.MarkTransfersAsProcessed([]string{transferID}); err != nil {
			t.Fatal(err)
		}

		status := getTransferStatus(t, repo, transferID)
		if status != client.PROCESSED {
			t.Errorf("unexpected transfer status: %s", status)
		}

		// error, unknown transferID
		if err := repo.MarkTransfersAsProcessed([]string{base.ID()}); err != nil {
			if !strings.Contains(err.Error(), "not found / updated") {
				t.Fatal(err)
			}
		} else {
			t.Error("expected error")
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

func setupSQLiteDB(t *testing.T) *sqlRepo {
	db := database.CreateTestSqliteDB(t)
	t.Cleanup(func() { db.Close() })

	return NewRepo(db.DB)
}

func setupMySQLeDB(t *testing.T) *sqlRepo {
	db := database.CreateTestMySQLDB(t)
	t.Cleanup(func() { db.Close() })

	return NewRepo(db.DB)
}

func writeMicroDeposit(t *testing.T, repo *sqlRepo, microDepositID, transferID string) {
	// Partial write into micro_deposits table -- just the fields we need.
	query := `insert into micro_deposits (micro_deposit_id, status) values (?, ?);`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(microDepositID, client.PENDING)
	if err != nil {
		t.Fatal(err)
	}

	// Write into micro-deposit table for linked transfers
	query = `insert into micro_deposit_transfers (micro_deposit_id, transfer_id) values (?, ?);`
	stmt, err = repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(microDepositID, transferID)
	if err != nil {
		t.Fatal(err)
	}
}

func writeTransfer(t *testing.T, repo *sqlRepo, transferID string) {
	// Partial write into transfers table -- just the fields we need.
	query := `insert into transfers (transfer_id, status) values (?, ?);`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(transferID, client.PENDING)
	if err != nil {
		t.Fatal(err)
	}
}

func getMicroDepositStatus(t *testing.T, repo *sqlRepo, microDepositID string) client.TransferStatus {
	query := `select status from micro_deposits where micro_deposit_id = ? limit 1;`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	var status client.TransferStatus
	if err := stmt.QueryRow(microDepositID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func getTransferStatus(t *testing.T, repo *sqlRepo, transferID string) client.TransferStatus {
	query := `select status from transfers where transfer_id = ? limit 1;`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	var status client.TransferStatus
	if err := stmt.QueryRow(transferID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/base/database"
	"github.com/moov-io/paygate/pkg/client"
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
			t.Errorf("unexpected micro-deposit xfer: %s", status)
		}

		xfer := getPartialTransferModel(t, repo, transferID)
		if xfer.Status != client.PROCESSED {
			t.Errorf("unexpected transfer status: %v", xfer.Status)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLDB(t))
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

		xfer := getPartialTransferModel(t, repo, transferID)
		if xfer.Status != client.PROCESSED {
			t.Errorf("unexpected transfer status: %s", xfer.Status)
		}
		if xfer.ProcessedAt == nil {
			t.Error("got nil ProcessedAt")
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
	check(t, setupMySQLDB(t))
}

func setupSQLiteDB(t *testing.T) *sqlRepo {
	db := database.CreateTestSQLiteDB(t)
	t.Cleanup(func() { db.Close() })

	return NewRepo(db.DB)
}

func setupMySQLDB(t *testing.T) *sqlRepo {
	db := database.CreateTestMySQLDB(t)
	t.Cleanup(func() { db.Close() })

	return NewRepo(db.DB)
}

func writeMicroDeposit(t *testing.T, repo *sqlRepo, microDepositID, transferID string) {
	micro := &client.MicroDeposits{
		MicroDepositID: microDepositID,
		Destination: client.Destination{
			CustomerID: base.ID(),
			AccountID:  base.ID(),
		},
		Amounts: []client.Amount{
			{Currency: "USD", Value: 2},
			{Currency: "USD", Value: 5},
		},
		Status:  client.PENDING,
		Created: time.Now(),
	}

	query := `insert into micro_deposits (micro_deposit_id, destination_customer_id, destination_account_id, status, created_at) values (?, ?, ?, ?, ?);`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(microDepositID, client.PENDING)
	_, err = stmt.Exec(
		micro.MicroDepositID,
		micro.Destination.CustomerID,
		micro.Destination.AccountID,
		micro.Status,
		micro.Created,
	)
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
	transfer := &client.Transfer{
		TransferID: transferID,
		Amount: client.Amount{
			Currency: "USD",
			Value:    1245,
		},
		Source: client.Source{
			CustomerID: base.ID(),
			AccountID:  base.ID(),
		},
		Destination: client.Destination{
			CustomerID: base.ID(),
			AccountID:  base.ID(),
		},
		Description: "payroll",
		Status:      client.PENDING,
		SameDay:     false,
		Created:     time.Now(),
	}

	query := `insert into transfers (transfer_id, organization, amount_currency, amount_value, source_customer_id, source_account_id, destination_customer_id, destination_account_id, description, status, same_day, created_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		transfer.TransferID,
		"orgID",
		transfer.Amount.Currency,
		transfer.Amount.Value,
		transfer.Source.CustomerID,
		transfer.Source.AccountID,
		transfer.Destination.CustomerID,
		transfer.Destination.AccountID,
		transfer.Description,
		transfer.Status,
		transfer.SameDay,
		time.Now(),
	)
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

func getPartialTransferModel(t *testing.T, repo *sqlRepo, transferID string) client.Transfer {
	query := `select status, processed_at from transfers where transfer_id = ? limit 1;`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	var xfer client.Transfer
	if err := stmt.QueryRow(transferID).Scan(&xfer.Status, &xfer.ProcessedAt); err != nil {
		t.Fatal(err)
	}
	return xfer
}

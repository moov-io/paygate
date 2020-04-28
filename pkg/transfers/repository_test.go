// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"net/http"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/database"
)

func TestRepository__getUserTransfers(t *testing.T) {
	userID := base.ID()
	repo := setupTestDatabase(t)
	writeTransfer(t, userID, repo)

	params := readTransferFilterParams(&http.Request{})
	xfers, err := repo.getUserTransfers(userID, params)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(xfers); n != 1 {
		t.Errorf("got %d transfers: %#v", n, xfers)
	}
}

func TestRepository__UpdateTransferStatus(t *testing.T) {
	userID := base.ID()
	repo := setupTestDatabase(t)

	xfer := writeTransfer(t, userID, repo)
	xfer, err := repo.GetTransfer(xfer.TransferID)
	if err != nil {
		t.Fatal(err)
	}
	if xfer.Status != client.PENDING {
		t.Fatalf("unexpected status: %v", xfer.Status)
	}

	if err := repo.UpdateTransferStatus(xfer.TransferID, client.CANCELED); err != nil {
		t.Fatal(err)
	}

	xfer, err = repo.GetTransfer(xfer.TransferID)
	if err != nil {
		t.Fatal(err)
	}
	if xfer.Status != client.CANCELED {
		t.Fatalf("unexpected status: %v", xfer.Status)
	}
}

func TestRepository__writeUserTransfers(t *testing.T) {
	userID := base.ID()
	repo := setupTestDatabase(t)

	xfer := writeTransfer(t, userID, repo)

	if tt, err := repo.GetTransfer(xfer.TransferID); err != nil {
		t.Fatal(err)
	} else {
		if tt.TransferID == "" {
			t.Errorf("missing transfer: %#v", tt)
		}
	}
}

func TestRepository__deleteUserTransfer(t *testing.T) {
	userID := base.ID()
	transferID := base.ID()
	repo := setupTestDatabase(t)

	if err := repo.deleteUserTransfer(userID, transferID); err != nil {
		t.Fatal(err)
	}

	xfer := writeTransfer(t, userID, repo)

	if err := repo.deleteUserTransfer(userID, xfer.TransferID); err != nil {
		t.Fatal(err)
	}
}

func setupTestDatabase(t *testing.T) *sqlRepo {
	db := database.CreateTestSqliteDB(t)
	t.Cleanup(func() { db.Close() })

	repo := &sqlRepo{db: db.DB}
	t.Cleanup(func() { repo.Close() })

	return repo
}

func writeTransfer(t *testing.T, userID string, repo Repository) *client.Transfer {
	t.Helper()

	xfer := &client.Transfer{
		TransferID: base.ID(),
		Amount:     "USD 12.45",
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

	if err := repo.writeUserTransfers(userID, xfer); err != nil {
		t.Fatal(err)
	}

	return xfer
}

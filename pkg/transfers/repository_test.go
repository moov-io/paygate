// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/database"
	"github.com/moov-io/paygate/pkg/model"
)

func TestRepository__getUserTransfers(t *testing.T) {
	userID := base.ID()
	repo := setupSQLiteDB(t)
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
	repo := setupSQLiteDB(t)

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

func TestRepository__WriteUserTransfer(t *testing.T) {
	userID := base.ID()
	repo := setupSQLiteDB(t)

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
	repo := setupSQLiteDB(t)

	if err := repo.deleteUserTransfer(userID, transferID); err != nil {
		t.Fatal(err)
	}

	// Write a PENDING transfer and delete it
	xfer := writeTransfer(t, userID, repo)
	if err := repo.deleteUserTransfer(userID, xfer.TransferID); err != nil {
		t.Fatal(err)
	}

	// Fail to delete a PROCESSED transfer
	xfer = writeTransfer(t, userID, repo)
	if err := repo.UpdateTransferStatus(xfer.TransferID, client.PROCESSED); err != nil {
		t.Fatal(err)
	}
	if err := repo.deleteUserTransfer(userID, xfer.TransferID); err != nil {
		if !strings.Contains(err.Error(), "is not in PENDING status") {
			t.Fatal(err)
		}
	} else {
		t.Error("expected error")
	}
}

func setupSQLiteDB(t *testing.T) *sqlRepo {
	db := database.CreateTestSqliteDB(t)
	t.Cleanup(func() { db.Close() })

	repo := &sqlRepo{db: db.DB}
	t.Cleanup(func() { repo.Close() })

	return repo
}

func setupMySQLeDB(t *testing.T) *sqlRepo {
	db := database.CreateTestMySQLDB(t)
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

	if err := repo.WriteUserTransfer(userID, xfer); err != nil {
		t.Fatal(err)
	}

	return xfer
}

func TestTransfers__SaveReturnCode(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		userID := base.ID()
		xfer := writeTransfer(t, userID, repo)

		// Set ReturnCode
		returnCode := "R17"
		if err := repo.SaveReturnCode(xfer.TransferID, returnCode); err != nil {
			t.Fatal(err)
		}

		xfer, err := repo.GetTransfer(xfer.TransferID)
		if err != nil {
			t.Fatal(err)
		}
		if xfer.ReturnCode.Code != returnCode {
			t.Errorf("xfer.ReturnCode=%q", xfer.ReturnCode)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

func TestTransfers__LookupTransferFromReturn(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		userID := base.ID()
		xfer := writeTransfer(t, userID, repo)

		// mark transfer as PROCESSED (which is usually set after upload)
		if err := repo.UpdateTransferStatus(xfer.TransferID, client.PROCESSED); err != nil {
			t.Fatal(err)
		}

		// save trace numbers for this Transfer
		if err := repo.saveTraceNumbers(xfer.TransferID, []string{"1234567"}); err != nil {
			t.Fatal(err)
		}

		// grab the transfer
		amt, _ := model.ParseAmount(xfer.Amount)
		found, err := repo.LookupTransferFromReturn(amt, "1234567", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if found == nil {
			t.Fatal("expected to find a Transfer")
		}

		if xfer.TransferID != found.TransferID {
			t.Errorf("unexpected transfer: %v", found.TransferID)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

func TestStartOfDayAndTomorrow(t *testing.T) {
	now := time.Now()
	min, max := startOfDayAndTomorrow(now)

	if !min.Before(now) {
		t.Errorf("min=%v now=%v", min, now)
	}
	if !max.After(now) {
		t.Errorf("max=%v now=%v", max, now)
	}

	if v := max.Sub(min); v != 24*time.Hour {
		t.Errorf("max - min = %v", v)
	}
}

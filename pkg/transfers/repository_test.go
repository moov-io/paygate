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

	"github.com/moov-io/base/database"
	"github.com/moov-io/paygate/pkg/client"
)

func TestRepository__getTransfers(t *testing.T) {
	orgID := base.ID()
	repo := setupSQLiteDB(t)
	writeTransfer(t, orgID, repo)

	params := readTransferFilterParams(&http.Request{})
	xfers, err := repo.getTransfers(orgID, params)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(xfers); n != 1 {
		t.Errorf("got %d transfers: %#v", n, xfers)
	}
	xferTraceNumbers := xfers[0].TraceNumbers
	if len(xferTraceNumbers) != 0 {
		t.Errorf("got %v traceNumbers:", xferTraceNumbers)
	}
}

func TestRepository__getTransfersWithTraceNumbers(t *testing.T) {
	orgID := base.ID()
	repo := setupSQLiteDB(t)
	transfer := writeTransfer(t, orgID, repo)
	traceNumbers := []string{
		"123",
		"456",
	}
	saveTraceNumbers(t, transfer, traceNumbers, repo)

	params := readTransferFilterParams(&http.Request{})
	xfers, err := repo.getTransfers(orgID, params)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(xfers); n != 1 {
		t.Errorf("got %d transfers: %#v", n, xfers)
	}
	xferTraceNumbers := xfers[0].TraceNumbers
	if len(xferTraceNumbers) != 2 {
		t.Errorf("got %v traceNumbers:", xferTraceNumbers)
	}
}

func TestRepository__getTransfersByStatus(t *testing.T) {
	orgID := "organization"
	repo := setupSQLiteDB(t)

	markedAsFailedIDs := make(map[string]bool)
	n := 10
	for i := 0; i < n; i++ {
		xfer := writeTransfer(t, orgID, repo)
		if i < n/2 {
			markedAsFailedIDs[xfer.TransferID] = true
		}
	}

	wantStatus := client.TransferStatus("failed")
	for id := range markedAsFailedIDs {
		err := repo.UpdateTransferStatus(id, wantStatus)
		if err != nil {
			t.Fatalf("updating transfer status: %v", err)
		}
	}

	params := readTransferFilterParams(&http.Request{})
	params.Status = wantStatus
	xfers, err := repo.getTransfers(orgID, params)
	if err != nil {
		t.Fatalf("getting transfers: %v", err)
	}

	if len(xfers) != len(markedAsFailedIDs) {
		t.Fatalf("number of transfers: want: %d, got: %d", len(markedAsFailedIDs), len(xfers))
	}

	for _, xfer := range xfers {
		_, ok := markedAsFailedIDs[xfer.TransferID]
		if !ok {
			t.Fatalf("transfer that is marked as %s is missing from results", wantStatus)
		}
	}
}

func TestRepository__getTransfersWithCustomerIDs(t *testing.T) {
	orgID := base.ID()
	repo := setupSQLiteDB(t)

	var xfers []*client.Transfer
	for i := 0; i < 10; i++ {
		xfer := writeTransfer(t, orgID, repo)
		xfers = append(xfers, xfer)
	}

	params := readTransferFilterParams(&http.Request{})
	params.CustomerIDs = []string{
		xfers[0].Source.CustomerID,
		xfers[1].Source.CustomerID,
		xfers[len(xfers)-2].Source.CustomerID,
		xfers[len(xfers)-1].Source.CustomerID,
	}
	got, err := repo.getTransfers(orgID, params)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(params.CustomerIDs) {
		t.Fatalf("# of transfers: want %d, got %d", len(params.CustomerIDs), len(got))
	}

	want := make(map[string]bool)
	for _, id := range params.CustomerIDs {
		want[id] = true
	}

	for _, xfer := range got {
		_, hasSourceID := want[xfer.Source.CustomerID]
		_, hasDestinationID := want[xfer.Destination.CustomerID]
		if !hasSourceID && !hasDestinationID {
			t.Fatal("customerID not found in source or destination")
		}
	}
}

func TestRepository__UpdateTransferStatus(t *testing.T) {
	orgID := base.ID()
	repo := setupSQLiteDB(t)

	xfer := writeTransfer(t, orgID, repo)
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
	orgID := base.ID()
	repo := setupSQLiteDB(t)

	xfer := writeTransfer(t, orgID, repo)
	if tt, err := repo.GetTransfer(xfer.TransferID); err != nil {
		t.Fatal(err)
	} else {
		if tt.TransferID == "" {
			t.Errorf("missing transfer: %#v", tt)
		}
	}
}

func TestRepository__deleteUserTransfer(t *testing.T) {
	orgID := base.ID()
	transferID := base.ID()
	repo := setupSQLiteDB(t)

	if err := repo.deleteUserTransfer(orgID, transferID); err != nil {
		t.Fatal(err)
	}

	// Write a PENDING transfer and delete it
	xfer := writeTransfer(t, orgID, repo)
	if err := repo.deleteUserTransfer(orgID, xfer.TransferID); err != nil {
		t.Fatal(err)
	}

	// Fail to delete a PROCESSED transfer
	xfer = writeTransfer(t, orgID, repo)
	if err := repo.UpdateTransferStatus(xfer.TransferID, client.PROCESSED); err != nil {
		t.Fatal(err)
	}
	if err := repo.deleteUserTransfer(orgID, xfer.TransferID); err != nil {
		if !strings.Contains(err.Error(), "is not in PENDING status") {
			t.Fatal(err)
		}
	} else {
		t.Error("expected error")
	}
}

func setupSQLiteDB(t *testing.T) *sqlRepo {
	db := database.CreateTestSQLiteDB(t)
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

func writeTransfer(t *testing.T, orgID string, repo Repository) *client.Transfer {
	t.Helper()

	xfer := &client.Transfer{
		TransferID: base.ID(),
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

	if err := repo.WriteUserTransfer(orgID, xfer); err != nil {
		t.Fatal(err)
	}

	return xfer
}

func saveTraceNumbers(t *testing.T, transfer *client.Transfer, traceNumbers []string, repo Repository) {
	if len(traceNumbers) > 0 {
		if err := repo.saveTraceNumbers(transfer.TransferID, traceNumbers); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTransfers__SaveReturnCode(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		orgID := base.ID()
		xfer := writeTransfer(t, orgID, repo)

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
		orgID := base.ID()
		xfer := writeTransfer(t, orgID, repo)

		// mark transfer as PROCESSED (which is usually set after upload)
		if err := repo.UpdateTransferStatus(xfer.TransferID, client.PROCESSED); err != nil {
			t.Fatal(err)
		}

		// save trace numbers for this Transfer
		if err := repo.saveTraceNumbers(xfer.TransferID, []string{"1234567"}); err != nil {
			t.Fatal(err)
		}

		// grab the transfer
		found, err := repo.LookupTransferFromReturn(xfer.Amount, "1234567", time.Now())
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

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"database/sql"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/config"

	"github.com/go-kit/kit/log"
)

func TestController__processReturnMicroDeposit(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "return-WEB.ach"))
	if err != nil {
		t.Fatal(err)
	}
	b := file.Batches[0]

	// Force the ReturnCode to a value we want for our tests
	b.GetEntries()[0].Addenda99.ReturnCode = "R02" // "Account Closed"

	amt, _ := internal.NewAmount("USD", "52.12")

	depRepo := &internal.MockDepositoryRepository{
		Depositories: []*internal.Depository{
			{
				ID:            internal.DepositoryID(base.ID()), // Don't use either DepositoryID from below
				BankName:      "my bank",
				Holder:        "jane doe",
				HolderType:    internal.Individual,
				Type:          internal.Savings,
				RoutingNumber: file.Header.ImmediateOrigin,
				AccountNumber: "123121",
				Status:        internal.DepositoryVerified,
				Metadata:      "other info",
			},
			{
				ID:            internal.DepositoryID(base.ID()), // Don't use either DepositoryID from below
				BankName:      "their bank",
				Holder:        "john doe",
				HolderType:    internal.Individual,
				Type:          internal.Savings,
				RoutingNumber: file.Header.ImmediateDestination,
				AccountNumber: b.GetEntries()[0].DFIAccountNumber,
				Status:        internal.DepositoryVerified,
				Metadata:      "other info",
			},
		},
		MicroDeposits: []*internal.MicroDeposit{
			{Amount: *amt},
		},
	}
	transferRepo := &internal.MockTransferRepository{
		Err: sql.ErrNoRows,
	}

	dir, _ := ioutil.TempDir("", "processReturnEntry")
	defer os.RemoveAll(dir)

	repo := newTestStaticRepository("ftp")

	controller, err := NewController(log.NewNopLogger(), config.Empty(), dir, repo, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0], depRepo, transferRepo); err != nil {
		t.Error(err)
	}

	// Check for our updated statuses
	if depRepo.Status != internal.DepositoryRejected {
		t.Errorf("Depository status wasn't updated, got %v", depRepo.Status)
	}
	if depRepo.ReturnCode != "R02" {
		t.Errorf("unexpected return code: %s", depRepo.ReturnCode)
	}
	if depRepo.Status != internal.DepositoryRejected {
		t.Errorf("unexpected status: %v", depRepo.Status)
	}

	// Check quick error conditions
	depRepo.Err = errors.New("bad error")
	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0], depRepo, transferRepo); err == nil {
		t.Error("expected error")
	}
	depRepo.Err = nil

	transferRepo.Err = errors.New("bad error")
	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0], depRepo, transferRepo); err == nil {
		t.Error("expected error")
	}
	transferRepo.Err = nil
}

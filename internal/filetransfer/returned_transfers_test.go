// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal"

	"github.com/go-kit/kit/log"
)

func TestController__processReturnTransfer(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "return-WEB.ach"))
	if err != nil {
		t.Fatal(err)
	}
	b := file.Batches[0]

	// Force the ReturnCode to a value we want for our tests
	b.GetEntries()[0].Addenda99.ReturnCode = "R02" // "Account Closed"

	amt, _ := internal.NewAmount("USD", "52.12")
	userID, transactionID := base.ID(), base.ID()

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
	}
	transferRepo := &internal.MockTransferRepository{
		Xfer: &internal.Transfer{
			Type:                   internal.PushTransfer,
			Amount:                 *amt,
			Originator:             internal.OriginatorID("originator"),
			OriginatorDepository:   internal.DepositoryID("orig-depository"),
			Receiver:               internal.ReceiverID("receiver"),
			ReceiverDepository:     internal.DepositoryID("rec-depository"),
			Description:            "transfer",
			StandardEntryClassCode: "PPD",
			UserID:                 userID,
			TransactionID:          transactionID,
		},
	}

	dir, _ := ioutil.TempDir("", "processReturnEntry")
	defer os.RemoveAll(dir)

	repo := newTestStaticRepository("ftp")

	controller, err := NewController(log.NewNopLogger(), dir, repo, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	// transferRepo.xfer will be returned inside processReturnEntry and the Transfer path will be executed
	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0], depRepo, transferRepo); err != nil {
		t.Error(err)
	}

	// Check for our updated statuses
	if depRepo.Status != internal.DepositoryRejected {
		t.Errorf("Depository status wasn't updated, got %v", depRepo.Status)
	}
	if transferRepo.ReturnCode != "R02" {
		t.Errorf("unexpected return code: %s", transferRepo.ReturnCode)
	}
	if transferRepo.Status != internal.TransferReclaimed {
		t.Errorf("unexpected status: %v", transferRepo.Status)
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

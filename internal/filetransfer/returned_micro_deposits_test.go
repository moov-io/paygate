// Copyright 2020 The Moov Authors
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
	appcfg "github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/depository/verification/microdeposit"
	"github.com/moov-io/paygate/internal/filetransfer/config"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/transfers"
	"github.com/moov-io/paygate/pkg/id"
)

func TestController__processReturnMicroDeposit(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "return-WEB.ach"))
	if err != nil {
		t.Fatal(err)
	}
	b := file.Batches[0]

	// Force the ReturnCode to a value we want for our tests
	b.GetEntries()[0].Addenda99.ReturnCode = "R02" // "Account Closed"

	amt, _ := model.NewAmount("USD", "52.12")

	depRepo := &depository.MockRepository{
		Depositories: []*model.Depository{
			{
				ID:                     id.Depository(base.ID()), // Don't use either DepositoryID from below
				BankName:               "my bank",
				Holder:                 "jane doe",
				HolderType:             model.Individual,
				Type:                   model.Savings,
				RoutingNumber:          file.Header.ImmediateOrigin,
				EncryptedAccountNumber: "123121",
				Status:                 model.DepositoryVerified,
				Metadata:               "other info",
			},
			{
				ID:                     id.Depository(base.ID()), // Don't use either DepositoryID from below
				BankName:               "their bank",
				Holder:                 "john doe",
				HolderType:             model.Individual,
				Type:                   model.Savings,
				RoutingNumber:          file.Header.ImmediateDestination,
				EncryptedAccountNumber: b.GetEntries()[0].DFIAccountNumber,
				Status:                 model.DepositoryVerified,
				Metadata:               "other info",
			},
		},
	}
	microDepositRepo := &microdeposit.MockRepository{
		Credits: []*microdeposit.Credit{
			{Amount: *amt},
		},
	}
	transferRepo := &transfers.MockRepository{
		Err: sql.ErrNoRows,
	}

	dir, _ := ioutil.TempDir("", "processReturnEntry")
	defer os.RemoveAll(dir)

	repo := config.NewRepository("", nil, "")

	cfg := appcfg.Empty()
	controller, err := NewController(cfg, dir, repo, depRepo, nil, microDepositRepo, nil, nil, transferRepo, makeTestODFIAccount(t), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0]); err != nil {
		t.Error(err)
	}

	// Check for our updated statuses
	if depRepo.Status != model.DepositoryRejected {
		t.Errorf("Depository status wasn't updated, got %v", depRepo.Status)
	}
	if rc := microDepositRepo.ReturnCode; rc != "R02" {
		t.Errorf("unexpected return code: %s", rc)
	}
	if depRepo.Status != model.DepositoryRejected {
		t.Errorf("unexpected status: %v", depRepo.Status)
	}

	// Check quick error conditions
	depRepo.Err = errors.New("bad error")
	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0]); err == nil {
		t.Error("expected error")
	}
	depRepo.Err = nil

	transferRepo.Err = errors.New("bad error")
	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0]); err == nil {
		t.Error("expected error")
	}
	transferRepo.Err = nil
}

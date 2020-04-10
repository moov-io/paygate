// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	appcfg "github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/filetransfer/admin"
	"github.com/moov-io/paygate/internal/filetransfer/config"
	"github.com/moov-io/paygate/internal/gateways"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/originators"
	"github.com/moov-io/paygate/internal/receivers"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/internal/transfers"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

func TestController__rejectRelatedObjects(t *testing.T) {
	logger := log.NewNopLogger()

	dir, _ := ioutil.TempDir("", "handleNOCFile")
	defer os.RemoveAll(dir)

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	repo := config.NewRepository("", nil, "")

	keeper := secrets.TestStringKeeper(t)
	depRepo := depository.NewDepositoryRepo(logger, sqliteDB.DB, keeper)
	transferRepo := &transfers.MockRepository{
		Xfer: &model.Transfer{
			ID: id.Transfer(base.ID()),
		},
	}
	gatewayRepo := &gateways.MockRepository{}
	originatorsRepo := &originators.MockRepository{}
	receiverRepo := &receivers.MockRepository{}

	cfg := appcfg.Empty()
	controller, err := NewController(cfg, dir, repo, depRepo, gatewayRepo, nil, originatorsRepo, receiverRepo, transferRepo, makeTestODFIAccount(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	controller.keeper = keeper
	controller.updateDepositoriesFromNOCs = false

	// read our test file and write it into the temp dir
	fd, err := os.Open(filepath.Join("..", "..", "testdata", "cor-c01.ach"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := ach.NewReader(fd).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.NotificationOfChange) == 0 {
		t.Fatalf("no NOC batches: %#v", file)
	}
	fd.Close()

	batch := file.NotificationOfChange[0]
	bh := batch.GetHeader()
	bh.EffectiveEntryDate = "190422"
	dep := &model.Depository{
		ID:     id.Depository(base.ID()),
		Status: model.DepositoryVerified,
	}

	// first and valid attempt at rejecting
	if err := controller.rejectRelatedObjects(bh, batch.GetEntries()[0], dep); err != nil {
		t.Errorf("got %v", err)
	}

	// transferRepo error
	transferRepo.Err = errors.New("bad error")
	if err := controller.rejectRelatedObjects(bh, batch.GetEntries()[0], dep); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "bad error") {
			t.Errorf("unexpected error: %v", err)
		}
	}
	transferRepo.Err = nil
	transferRepo.Xfer = nil
	if err := controller.rejectRelatedObjects(bh, batch.GetEntries()[0], dep); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "transfer not found") {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// depRepo error
	sqliteDB.Close()
	if err := controller.rejectRelatedObjects(bh, batch.GetEntries()[0], dep); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "database is closed") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

// depositoryChangeCode writes a Depository and then calls updateDepositoryFromChangeCode given the provided change code.
// The Depository is then re-read and returned from this method
func depositoryChangeCode(t *testing.T, controller *Controller, changeCode string) (*model.Depository, error) {
	logger := log.NewNopLogger()

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	keeper := secrets.TestStringKeeper(t)
	repo := depository.NewDepositoryRepo(logger, sqliteDB.DB, keeper)

	userID := id.User(base.ID())
	dep := &model.Depository{
		ID:       id.Depository(base.ID()),
		BankName: "my bank",
		Status:   model.DepositoryVerified,
	}
	if err := repo.UpsertUserDepository(userID, dep); err != nil {
		return nil, err
	}

	ed := &ach.EntryDetail{
		Addenda98: &ach.Addenda98{
			ChangeCode: changeCode,
			CorrectedData: ach.WriteCorrectionData(changeCode, &ach.CorrectedData{
				RoutingNumber:   "987654320",
				AccountNumber:   "1242415",
				TransactionCode: ach.CheckingCredit,
			}),
		},
	}
	cc := &ach.ChangeCode{Code: changeCode}

	if err := controller.updateDepositoryFromChangeCode(cc, ed, dep); err != nil {
		return nil, err
	}

	dep, _ = repo.GetUserDepository(dep.ID, userID)
	return dep, nil
}

func TestDepositories__updateDepositoryFromChangeCode(t *testing.T) {
	dir, _ := ioutil.TempDir("", "handleNOCFile")
	defer os.RemoveAll(dir)

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	repo := config.NewRepository("", nil, "")

	keeper := secrets.TestStringKeeper(t)
	depRepo := depository.NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper)

	cfg := appcfg.Empty()
	controller, err := NewController(cfg, dir, repo, depRepo, nil, nil, nil, nil, nil, makeTestODFIAccount(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	controller.keeper = keeper
	controller.updateDepositoriesFromNOCs = true

	cases := []struct {
		code     string
		expected model.DepositoryStatus
	}{
		{"C05", model.DepositoryRejected},
		{"C06", model.DepositoryRejected},
		{"C07", model.DepositoryRejected},
	}
	for i := range cases {
		dep, err := depositoryChangeCode(t, controller, cases[i].code)
		if dep == nil || err != nil {
			if !strings.Contains(err.Error(), "rejecting originalTrace") && !strings.Contains(err.Error(), "after new transactionCode") {
				t.Fatalf("code=%s depository=%#v error=%v", cases[i].code, dep, err)
			}
			continue // next case
		}
		if dep.Status != cases[i].expected {
			t.Errorf("%s: dep.Status=%v", cases[i].code, dep.Status)
		}
	}
}

func TestController__handleNOCFile(t *testing.T) {
	userID := id.User(base.ID())
	logger := log.NewNopLogger()
	dir, _ := ioutil.TempDir("", "handleNOCFile")
	defer os.RemoveAll(dir)

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	repo := config.NewRepository("", nil, "")

	keeper := secrets.TestStringKeeper(t)
	depRepo := depository.NewDepositoryRepo(logger, sqliteDB.DB, keeper)

	cfg := appcfg.Empty()
	controller, err := NewController(cfg, dir, repo, depRepo, nil, nil, nil, nil, nil, makeTestODFIAccount(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	controller.keeper = keeper
	controller.updateDepositoriesFromNOCs = true

	// read our test file and write it into the temp dir
	fd, err := os.Open(filepath.Join("..", "..", "testdata", "cor-c01.ach"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := ach.NewReader(fd).Read()
	if err != nil {
		t.Fatal(err)
	}
	fd.Close()

	// write the Depository
	dep := &model.Depository{
		ID:            id.Depository(base.ID()),
		RoutingNumber: file.Header.ImmediateDestination,
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    model.Individual,
		Type:          model.Checking,
		Status:        model.DepositoryVerified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}
	dep, _ = depRepo.GetDepository(dep.ID) // this method sets the keeper

	accountNumber := strings.TrimSpace(file.Batches[0].GetEntries()[0].DFIAccountNumber)
	if err := dep.ReplaceAccountNumber(accountNumber); err != nil {
		t.Fatal(err)
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil { // write encrypted account number
		t.Fatal(err)
	}

	// run the controller
	req := &admin.Request{}
	if err := controller.handleNOCFile(req, &file, "cor-c01.ach"); err != nil {
		t.Error(err)
	}

	// check the Depository status
	dep, err = depRepo.GetUserDepository(dep.ID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if dep.Status != model.DepositoryVerified {
		t.Errorf("dep.Status=%s", dep.Status)
	}

	t.Logf("dep=%#v", dep)

	// verify account number was changed
	if dep.EncryptedAccountNumber == "" {
		t.Fatal("empty encrypted account number")
	}
	if num, err := dep.DecryptAccountNumber(); err != nil {
		t.Fatal(err)
	} else {
		if num != "1918171614" {
			t.Errorf("account number: %s", num)
		}
	}
}

func TestController__handleNOCFileEmpty(t *testing.T) {
	dir, _ := ioutil.TempDir("", "handleNOCFile")
	defer os.RemoveAll(dir)

	repo := config.NewRepository("", nil, "")

	cfg := appcfg.Empty()
	controller, err := NewController(cfg, dir, repo, nil, nil, nil, nil, nil, nil, makeTestODFIAccount(t), nil)
	if err != nil {
		t.Fatal(err)
	}

	// read a non-NOC file to skip its handling
	fd, err := os.Open(filepath.Join("..", "..", "testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := ach.NewReader(fd).Read()
	if err != nil {
		t.Fatal(err)
	}
	fd.Close()

	// handoff the file but watch it be skipped
	req := &admin.Request{}
	if err := controller.handleNOCFile(req, &file, "ppd-debit.ach"); err != nil {
		t.Error(err)
	}

	// fake a NotificationOfChange array item (but it's missing Addenda98)
	file.NotificationOfChange = append(file.NotificationOfChange, file.Batches[0])
	if err := controller.handleNOCFile(req, &file, "foo.ach"); err != nil {
		t.Error(err)
	}
}

func TestCorrectionsErr__updateDepositoryFromChangeCode(t *testing.T) {
	userID := id.User(base.ID())
	logger := log.NewNopLogger()

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	repo := config.NewRepository("", nil, "")

	cc := &ach.ChangeCode{Code: "C14"}
	ed := &ach.EntryDetail{Addenda98: &ach.Addenda98{}}

	cfg := appcfg.Empty()

	dir, err := ioutil.TempDir("", "Controller")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	keeper := secrets.TestStringKeeper(t)
	depRepo := depository.NewDepositoryRepo(logger, sqliteDB.DB, keeper)

	controller, _ := NewController(cfg, dir, repo, depRepo, nil, nil, nil, nil, nil, makeTestODFIAccount(t), nil)
	controller.keeper = keeper
	controller.updateDepositoriesFromNOCs = true

	if err := controller.updateDepositoryFromChangeCode(cc, ed, nil); err == nil {
		t.Error("nil Depository, expected error")
	} else {
		if !strings.Contains(err.Error(), "depository not found") {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// test an unexpected change code
	dep := &model.Depository{
		ID:                     id.Depository(base.ID()),
		RoutingNumber:          "987654320",
		EncryptedAccountNumber: "4512",
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	// not implemented change code
	cc.Code = "C04"
	ed.Addenda98.ChangeCode = cc.Code
	ed.Addenda98.CorrectedData = ach.WriteCorrectionData(cc.Code, &ach.CorrectedData{
		Name: "john smith",
	})
	if err := controller.updateDepositoryFromChangeCode(cc, ed, dep); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "skipping receiver individual name") {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// unknown change code
	cc.Code = "C99"
	ed.Addenda98.CorrectedData = ""
	if err := controller.updateDepositoryFromChangeCode(cc, ed, dep); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "missing Addenda98 record") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

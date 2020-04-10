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

	"github.com/moov-io/base"
	appcfg "github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/depository/verification/microdeposit"
	"github.com/moov-io/paygate/internal/filetransfer/config"
	"github.com/moov-io/paygate/internal/gateways"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/originators"
	"github.com/moov-io/paygate/internal/receivers"
	"github.com/moov-io/paygate/internal/transfers"
	"github.com/moov-io/paygate/pkg/id"
)

func TestController__handleRemoval(t *testing.T) {
	dir, err := ioutil.TempDir("", "handleRemoval")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := appcfg.Empty()
	controller, err := NewController(cfg, dir, nil, nil, nil, nil, nil, nil, nil, makeTestODFIAccount(t), nil)
	if err != nil {
		t.Fatal(err)
	}

	// nil message, make sure we don't panic
	controller.handleRemoval(nil)
}

func TestController__removeMicroDepositErr(t *testing.T) {
	dir, err := ioutil.TempDir("", "removeMicroDepositErr")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := appcfg.Empty()
	repo := config.NewRepository("", nil, "")
	depRepo := &depository.MockRepository{}
	gatewayRepo := &gateways.MockRepository{}
	microDepositRepo := &microdeposit.MockRepository{}
	originatorsRepo := &originators.MockRepository{}
	receiverRepo := &receivers.MockRepository{}
	transferRepo := &transfers.MockRepository{}

	controller, err := NewController(cfg, dir, repo, depRepo, gatewayRepo, microDepositRepo, originatorsRepo, receiverRepo, transferRepo, makeTestODFIAccount(t), nil)
	if err != nil {
		t.Fatal(err)
	}

	depID := id.Depository(base.ID())
	req := &depository.RemoveMicroDeposits{
		DepositoryID: depID,
	}

	microDepositRepo.Err = errors.New("bad error")
	if err := controller.removeMicroDeposit(req); err == nil {
		t.Error("expected error")
	}
	microDepositRepo.Err = nil
	if err := controller.removeMicroDeposit(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestController__removeTransferErr(t *testing.T) {
	dir, err := ioutil.TempDir("", "Controller")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	repo := config.NewRepository("", nil, "")

	cfg := appcfg.Empty()

	depID := id.Depository(base.ID())
	depRepo := &depository.MockRepository{
		Depositories: []*model.Depository{
			{
				ID:            depID,
				RoutingNumber: "987654320",
			},
		},
	}
	gatewayRepo := &gateways.MockRepository{}
	microDepositRepo := &microdeposit.MockRepository{}
	originatorsRepo := &originators.MockRepository{}
	receiverRepo := &receivers.MockRepository{}
	transferRepo := &transfers.MockRepository{
		FileID: base.ID(),
	}

	controller, err := NewController(cfg, dir, repo, depRepo, gatewayRepo, microDepositRepo, originatorsRepo, receiverRepo, transferRepo, makeTestODFIAccount(t), nil)
	if err != nil {
		t.Fatal(err)
	}

	req := &transfers.RemoveTransferRequest{
		Transfer: &model.Transfer{
			ID:                 id.Transfer(base.ID()),
			ReceiverDepository: depID,
		},
	}

	// First error condition
	transferRepo.Err = errors.New("bad error")
	err = controller.removeTransfer(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "missing fileID for transfer") {
		t.Errorf("unexpected error: %v", err)
	}
	transferRepo.Err = nil
	transferRepo.FileID = "fileID"

	// Third error condition
	depRepo.Err = errors.New("bad error")
	err = controller.removeTransfer(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "missing receiver depository") {
		t.Errorf("unexpected error: %v", err)
	}
	depRepo.Err = nil

	// handleRemoval error, make sure we don't panic
	transferRepo.Err = errors.New("bad error")
	controller.handleRemoval(req)
	// no error, make sure we don't panic
	transferRepo.Err = nil
	controller.handleRemoval(req)
}

func TestController__removeBatchSingle(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := ioutil.TempDir("", "removeBatchSingle")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	mergable := &achFile{
		File:     file,
		filepath: filepath.Join(dir, "076401251.ach"),
	}
	if err := mergable.write(); err != nil {
		t.Fatal(err)
	}
	if err := removeBatch(mergable, "076401255655291"); err != nil {
		t.Fatal(err)
	}

	fds, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fds) != 0 {
		t.Errorf("found %d fds: %#v", len(fds), fds)
	}
}

func TestController__removeBatchMulti(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "return-WEB.ach"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := ioutil.TempDir("", "removeBatchMulti")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	mergable := &achFile{
		File:     file,
		filepath: filepath.Join(dir, "091400606.ach"),
	}
	if err := mergable.write(); err != nil {
		t.Fatal(err)
	}
	if err := removeBatch(mergable, "021000029461242"); err != nil {
		t.Fatal(err)
	}

	fds, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fds) != 1 {
		t.Errorf("found %d fds: %#v", len(fds), fds)
	}

	file, err = parseACHFilepath(mergable.filepath)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Batches) != 1 {
		t.Errorf("%d batches: %#v", len(file.Batches), file)
	}

	// missing TraceNumber
	if err := removeBatch(mergable, "666ff60c"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

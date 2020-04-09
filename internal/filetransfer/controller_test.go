// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/paygate/internal/accounts"
	appcfg "github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/depository/verification/microdeposit"
	"github.com/moov-io/paygate/internal/filetransfer/admin"
	"github.com/moov-io/paygate/internal/filetransfer/config"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/internal/transfers"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

type TestController struct {
	*Controller

	dir string

	repo             *config.StaticRepository
	depRepo          *depository.MockRepository
	microDepositRepo *microdeposit.MockRepository
	transferRepo     *transfers.MockRepository

	accountsClient accounts.Client
}

func (c *TestController) Close() {
	if c == nil {
		return
	}
	os.RemoveAll(c.dir)
}

func setupTestController(t *testing.T) *TestController {
	t.Helper()

	cfg := appcfg.Empty()
	cfg.Logger = log.NewLogfmtLogger(os.Stdout)
	dir, _ := ioutil.TempDir("", "file-transfer-controller")

	repo := &config.StaticRepository{}
	repo.Populate()

	// {
	// 	RoutingNumber: "121042882",
	// 	InboundPath:   "inbound/",
	// 	OutboundPath:  "outbound/",
	// 	ReturnPath:    "returned/",
	// },
	// {
	// 	RoutingNumber: "076401251",
	// 	InboundPath:   "inbound/",
	// 	OutboundPath:  "outbound/",
	// 	ReturnPath:    "returned/",
	// },

	depRepo := &depository.MockRepository{}
	microDepositRepo := &microdeposit.MockRepository{}
	transferRepo := &transfers.MockRepository{}

	accountsClient := &accounts.MockClient{}

	controller, err := NewController(cfg, dir, repo, depRepo, microDepositRepo, transferRepo, accountsClient)
	if err != nil {
		t.Fatal(err)
	}

	out := &TestController{
		Controller:       controller,
		dir:              dir,
		repo:             repo,
		depRepo:          depRepo,
		microDepositRepo: microDepositRepo,
		transferRepo:     transferRepo,
		accountsClient:   accountsClient,
	}
	t.Cleanup(func() { out.Close() })
	return out
}

func TestController__cutoffs(t *testing.T) {
	dir, err := ioutil.TempDir("", "Controller")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	repo := config.NewRepository("", nil, "")

	cfg := appcfg.Empty()
	controller, err := NewController(cfg, dir, repo, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v := fmt.Sprintf("%v", controller.interval); v != "10m0s" {
		t.Errorf("interval, got %q", v)
	}
	if controller.batchSize != 100 {
		t.Errorf("batchSize: %d", controller.batchSize)
	}

	cutoffTimes, err := controller.repo.GetCutoffTimes()
	if len(cutoffTimes) != 1 || err != nil {
		t.Errorf("local len(cutoffTimes)=%d error=%v", len(cutoffTimes), err)
	}
	ftpConfigs, err := controller.repo.GetFTPConfigs()
	if len(ftpConfigs) != 1 || err != nil {
		t.Errorf("local len(ftpConfigs)=%d error=%v", len(ftpConfigs), err)
	}

	if r, ok := controller.repo.(*config.StaticRepository); ok {
		r.Protocol = "sftp" // force into SFTP mode
		r.Populate()
	} else {
		t.Fatalf("got %#v", controller.repo)
	}
	sftpConfigs, err := controller.repo.GetSFTPConfigs()
	if len(sftpConfigs) != 1 || err != nil {
		t.Errorf("local len(sftpConfigs)=%d error=%v", len(sftpConfigs), err)
	}
}

func TestController__findFileTransferConfig(t *testing.T) {
	cutoff := &config.CutoffTime{
		RoutingNumber: "123",
		Cutoff:        1700,
		Loc:           time.UTC,
	}

	controller := setupTestController(t)
	controller.repo.Configs = []*config.Config{
		{RoutingNumber: "123", InboundPath: "inbound/"},
		{RoutingNumber: "321", InboundPath: "incoming/"},
	}
	controller.repo.FTPConfigs = []*config.FTPConfig{
		{RoutingNumber: "123", Hostname: "ftp.foo.com"},
		{RoutingNumber: "321", Hostname: "ftp.bar.com"},
	}

	// happy path - found
	fileTransferConf := controller.findFileTransferConfig(cutoff.RoutingNumber)
	if fileTransferConf == nil {
		t.Fatalf("fileTransferConf=%v", fileTransferConf)
	}
	if fileTransferConf.InboundPath != "inbound/" {
		t.Errorf("fileTransferConf=%#v", fileTransferConf)
	}

	// not found
	fileTransferConf = controller.findFileTransferConfig("456")
	if fileTransferConf != nil {
		t.Fatalf("fileTransferConf=%v", fileTransferConf)
	}

	// error
	controller.repo.Err = errors.New("bad errors")
	if conf := controller.findFileTransferConfig("987654320"); conf != nil {
		t.Error("expected nil config")
	}
}

func TestController__findAgentType(t *testing.T) {
	controller := setupTestController(t)

	if v := controller.findAgentType(""); v != "unknown" {
		t.Errorf("got %s", v)
	}
	if v := controller.findAgentType("987654320"); v != "unknown" {
		t.Errorf("got %s", v)
	}

	// Get 'sftp' as type
	controller.repo.SFTPConfigs = []*config.SFTPConfig{
		{RoutingNumber: "987654320"},
	}
	if v := controller.findAgentType("987654320"); v != "sftp" {
		t.Errorf("got %s", v)
	}

	// 'ftp' is checked first, so let's override that now
	controller.repo.FTPConfigs = []*config.FTPConfig{
		{RoutingNumber: "987654320"},
	}
	if v := controller.findAgentType("987654320"); v != "ftp" {
		t.Errorf("got %s", v)
	}

	// error
	controller.repo.Err = errors.New("bad error")
	if v := controller.findAgentType("ftp"); !strings.Contains(v, "unknown: error") {
		t.Errorf("got %s", v)
	}
}

func TestController__startPeriodicFileOperations(t *testing.T) {
	// FYI, this test is more about bumping up code coverage than testing anything.
	// How the polling loop is implemented currently prevents us from inspecting much
	// about what it does.

	dir, _ := ioutil.TempDir("", "startPeriodicFileOperations")
	defer os.RemoveAll(dir)

	repo := config.NewRepository("", nil, "")

	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	keeper := secrets.TestStringKeeper(t)
	innerDepRepo := depository.NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)
	microDepositRepo := &microdeposit.MockRepository{}
	microDepositRepo.Cur = &microdeposit.Cursor{
		BatchSize: 5,
		Repo:      microdeposit.NewRepository(log.NewNopLogger(), db.DB),
	}
	transferRepo := &transfers.MockRepository{
		Cur: &transfers.Cursor{
			BatchSize:    5,
			TransferRepo: transfers.NewTransferRepo(log.NewNopLogger(), db.DB),
		},
	}

	// write a micro-deposit
	amt, _ := model.NewAmount("USD", "0.22")
	if err := microDepositRepo.InitiateMicroDeposits(id.Depository("depositoryID"), "userID", []*microdeposit.Credit{{Amount: *amt, FileID: "fileID"}}); err != nil {
		t.Fatal(err)
	}

	// setup transfer controller to start a manual merge and upload
	cfg := appcfg.Empty()
	controller, err := NewController(cfg, dir, repo, innerDepRepo, microDepositRepo, transferRepo, nil)
	if err != nil {
		t.Fatal(err)
	}

	flushIncoming, flushOutgoing := make(admin.FlushChan, 1), make(admin.FlushChan, 1)
	removal := make(RemovalChan, 1)
	ctx, cancelFileSync := context.WithCancel(context.Background())

	go controller.StartPeriodicFileOperations(ctx, flushIncoming, flushOutgoing, removal) // async call to register the polling loop
	// trigger the calls
	flushIncoming <- &admin.Request{}
	flushOutgoing <- &admin.Request{}

	time.Sleep(250 * time.Millisecond)

	cancelFileSync()
}

func readFileAsCloser(path string) io.ReadCloser {
	fd, err := os.Open(path)
	if err != nil {
		return nil
	}
	bs, _ := ioutil.ReadAll(fd)
	return ioutil.NopCloser(bytes.NewReader(bs))
}

func TestController__ACHFile(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("nil ach.File")
	}

	dir, err := ioutil.TempDir("", "paygate")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// test writing the file
	f := &achFile{
		File:     file,
		filepath: filepath.Join(dir, "out.ach"),
	}
	if err := f.write(); err != nil {
		t.Fatal(err)
	}
	if fd, err := os.Stat(f.filepath); err != nil || fd.Size() == 0 {
		t.Fatalf("fd=%v err=%v", fd, err)
	}
	if n := f.lineCount(); n != 10 {
		t.Errorf("got %d for line count", n)
	}
}

func writeACHFile(path string) error {
	fd, err := os.Open(filepath.Join("..", "..", "testdata", "ppd-debit.ach"))
	if err != nil {
		return err
	}
	defer fd.Close()
	f, err := parseACHFile(fd)
	if err != nil {
		return err
	}
	return (&achFile{
		File:     f,
		filepath: path,
	}).write()
}

// Copyright 2019 The Moov Authors
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
	"sync"
	"testing"
	"time"

	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestController(t *testing.T) {
	dir, err := ioutil.TempDir("", "Controller")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	repo := newTestStaticRepository("ftp")

	controller, err := NewController(log.NewNopLogger(), dir, repo, nil, nil, true)
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

	// force the repository into SFTP mode
	if r, ok := controller.repo.(*staticRepository); ok {
		r.protocol = "sftp"
		r.populateSFTPConfigs()
	} else {
		t.Fatalf("got %#v", controller.repo)
	}
	sftpConfigs, err := controller.repo.GetSFTPConfigs()
	if len(sftpConfigs) != 1 || err != nil {
		t.Errorf("local len(sftpConfigs)=%d error=%v", len(sftpConfigs), err)
	}
}

func TestController__findFileTransferConfig(t *testing.T) {
	cutoff := &CutoffTime{
		RoutingNumber: "123",
		Cutoff:        1700,
		Loc:           time.UTC,
	}
	repo := &mockRepository{
		configs: []*Config{
			{RoutingNumber: "123", InboundPath: "inbound/"},
			{RoutingNumber: "321", InboundPath: "incoming/"},
		},
		ftpConfigs: []*FTPConfig{
			{RoutingNumber: "123", Hostname: "ftp.foo.com"},
			{RoutingNumber: "321", Hostname: "ftp.bar.com"},
		},
	}
	controller := &Controller{repo: repo}

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
	repo.err = errors.New("bad errors")
	if conf := controller.findFileTransferConfig("987654320"); conf != nil {
		t.Error("expected nil config")
	}
}

func TestController__findTransferType(t *testing.T) {
	controller := &Controller{
		repo: &mockRepository{},
	}

	if v := controller.findTransferType(""); v != "unknown" {
		t.Errorf("got %s", v)
	}
	if v := controller.findTransferType("987654320"); v != "unknown" {
		t.Errorf("got %s", v)
	}

	// Get 'sftp' as type
	controller = &Controller{
		repo: &mockRepository{
			sftpConfigs: []*SFTPConfig{
				{RoutingNumber: "987654320"},
			},
		},
	}
	if v := controller.findTransferType("987654320"); v != "sftp" {
		t.Errorf("got %s", v)
	}

	// 'ftp' is checked first, so let's override that now
	controller = &Controller{
		repo: &mockRepository{
			ftpConfigs: []*FTPConfig{
				{RoutingNumber: "987654320"},
			},
		},
	}
	if v := controller.findTransferType("987654320"); v != "ftp" {
		t.Errorf("got %s", v)
	}

	// error
	controller = &Controller{
		repo: &mockRepository{
			err: errors.New("bad error"),
		},
	}
	if v := controller.findTransferType("ftp"); !strings.Contains(v, "unknown: error") {
		t.Errorf("got %s", v)
	}
}

func TestController__startPeriodicFileOperations(t *testing.T) {
	// FYI, this test is more about bumping up code coverage than testing anything.
	// How the polling loop is implemented currently prevents us from inspecting much
	// about what it does.

	logger := log.NewNopLogger()

	dir, _ := ioutil.TempDir("", "startPeriodicFileOperations")
	defer os.RemoveAll(dir)

	repo := newTestStaticRepository("ftp")

	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	innerDepRepo := internal.NewDepositoryRepo(log.NewNopLogger(), db.DB)
	depRepo := &internal.MockDepositoryRepository{
		Cur: &internal.MicroDepositCursor{
			BatchSize: 5,
			DepRepo:   innerDepRepo,
		},
	}
	transferRepo := &internal.MockTransferRepository{
		Cur: &internal.TransferCursor{
			BatchSize:    5,
			TransferRepo: internal.NewTransferRepo(log.NewNopLogger(), db.DB),
		},
	}

	// write a micro-deposit
	amt, _ := internal.NewAmount("USD", "0.22")
	if err := innerDepRepo.InitiateMicroDeposits(internal.DepositoryID("depositoryID"), "userID", []*internal.MicroDeposit{{Amount: *amt, FileID: "fileID"}}); err != nil {
		t.Fatal(err)
	}

	achClient, _, achServer := achclient.MockClientServer("mergeGroupableTransfer", func(r *mux.Router) {
		achFileContentsRoute(r)
	})
	defer achServer.Close()

	// setuo transfer controller to start a manual merge and upload
	controller, err := NewController(logger, dir, repo, achClient, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	flushIncoming, flushOutgoing := make(FlushChan, 1), make(FlushChan, 1)
	ctx, cancelFileSync := context.WithCancel(context.Background())

	go controller.StartPeriodicFileOperations(ctx, flushIncoming, flushOutgoing, depRepo, transferRepo) // async call to register the polling loop
	// trigger the calls
	flushIncoming <- &periodicFileOperationsRequest{}
	flushOutgoing <- &periodicFileOperationsRequest{}

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

type mockFileTransferAgent struct {
	inboundFiles []File
	returnFiles  []File
	uploadedFile *File        // non-nil on file upload
	deletedFile  string       // filepath of last deleted file
	mu           sync.RWMutex // protects all fields
}

func (a *mockFileTransferAgent) GetInboundFiles() ([]File, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.inboundFiles, nil
}

func (a *mockFileTransferAgent) GetReturnFiles() ([]File, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.returnFiles, nil
}

func (a *mockFileTransferAgent) UploadFile(f File) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// read f.contents before callers close the underlying os.Open file descriptor
	bs, _ := ioutil.ReadAll(f.Contents)
	a.uploadedFile = &f
	a.uploadedFile.Contents = ioutil.NopCloser(bytes.NewReader(bs))
	return nil
}

func (a *mockFileTransferAgent) Delete(path string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.deletedFile = path
	return nil
}

func (a *mockFileTransferAgent) InboundPath() string  { return "inbound/" }
func (a *mockFileTransferAgent) OutboundPath() string { return "outbound/" }
func (a *mockFileTransferAgent) ReturnPath() string   { return "return/" }

func (a *mockFileTransferAgent) Close() error { return nil }

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

func TestACHFile__removeBatch(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	af := &achFile{File: file}
	af.removeBatch(file.Batches[0])
	if len(af.Batches) != 0 {
		t.Errorf("got %d batches", len(af.Batches))
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

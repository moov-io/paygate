// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func NestFileTransferController__newFileTransferController(t *testing.T) {
	dir, err := ioutil.TempDir("", "fileTransferController")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	repo := NewRepository(nil, "local") // localFileTransferRepository

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
	if len(controller.cutoffTimes) != 1 {
		t.Errorf("local len(controller.cutoffTimes)=%d", len(controller.cutoffTimes))
	}
	if len(controller.sftpConfigs) != 0 {
		t.Errorf("local len(controller.sftpConfigs)=%d", len(controller.sftpConfigs))
	}
	if len(controller.fileTransferConfigs) != 1 {
		t.Errorf("local len(controller.fileTransferConfigs)=%d", len(controller.fileTransferConfigs))
	}
}

func TestFileTransferController__findFileTransferConfig(t *testing.T) {
	cutoff := &CutoffTime{
		RoutingNumber: "123",
		Cutoff:        1700,
		Loc:           time.UTC,
	}
	controller := &fileTransferController{
		ftpConfigs: []*FTPConfig{
			{
				RoutingNumber: "123",
				Hostname:      "ftp.foo.com",
			},
			{
				RoutingNumber: "321",
				Hostname:      "ftp.bar.com",
			},
		},
		fileTransferConfigs: []*Config{
			{
				RoutingNumber: "123",
				InboundPath:   "inbound/",
			},
			{
				RoutingNumber: "321",
				InboundPath:   "incoming/",
			},
		},
	}

	// happy path - found
	fileTransferConf := controller.findFileTransferConfig(cutoff)
	if fileTransferConf == nil {
		t.Fatalf("fileTransferConf=%v", fileTransferConf)
	}
	if fileTransferConf.InboundPath != "inbound/" {
		t.Errorf("fileTransferConf=%#v", fileTransferConf)
	}

	// not found
	fileTransferConf = controller.findFileTransferConfig(&CutoffTime{RoutingNumber: "456"})
	if fileTransferConf != nil {
		t.Fatalf("fileTransferConf=%v", fileTransferConf)
	}
}

func TestFileTransferController__findTransferType(t *testing.T) {
	controller := &fileTransferController{}

	if v := controller.findTransferType(""); v != "unknown" {
		t.Errorf("got %s", v)
	}
	if v := controller.findTransferType("987654320"); v != "unknown" {
		t.Errorf("got %s", v)
	}

	// Get 'sftp' as type
	controller.sftpConfigs = append(controller.sftpConfigs, &SFTPConfig{
		RoutingNumber: "987654320",
	})
	if v := controller.findTransferType("987654320"); v != "sftp" {
		t.Errorf("got %s", v)
	}

	// 'ftp' is checked first, so let's override that now
	controller.ftpConfigs = append(controller.ftpConfigs, &FTPConfig{
		RoutingNumber: "987654320",
	})
	if v := controller.findTransferType("987654320"); v != "ftp" {
		t.Errorf("got %s", v)
	}
}

func TestFileTransferController__startPeriodicFileOperations(t *testing.T) {
	// FYI, this test is more about bumping up code coverage than testing anything.
	// How the polling loop is implemented currently prevents us from inspecting much
	// about what it does.

	logger := log.NewNopLogger()

	dir, _ := ioutil.TempDir("", "startPeriodicFileOperations")
	defer os.RemoveAll(dir)

	repo := NewRepository(nil, "local") // localFileTransferRepository

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

	flushIncoming, flushOutgoing := make(chan struct{}, 1), make(chan struct{}, 1)
	ctx, cancelFileSync := context.WithCancel(context.Background())

	go controller.StartPeriodicFileOperations(ctx, flushIncoming, flushOutgoing, depRepo, transferRepo) // async call to register the polling loop
	flushIncoming <- struct{}{}                                                                         // trigger the calls
	flushOutgoing <- struct{}{}

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

func TestFileTransferController__achFilename(t *testing.T) {
	now := time.Now().Format("20060102")

	if v := achFilename("12345789", 2); v != fmt.Sprintf("%s-12345789-2.ach", now) {
		t.Errorf("got %q", v)
	}
	if n := achFilenameSeq(achFilename("12345789", 2)); n != 2 {
		t.Errorf("got %d", n)
	}
	if v := achFilenameSeqToStr(3); v != "3" {
		t.Errorf("got %s", v)
	}

	// test wrap around to A-Z
	if v := achFilename("123456789", 10); v != fmt.Sprintf("%s-123456789-A.ach", now) {
		t.Errorf("got %q", v)
	}
	if v := achFilename("123456789", 12); v != fmt.Sprintf("%s-123456789-C.ach", now) {
		t.Errorf("got %q", v)
	}
	if n := achFilenameSeq(achFilename("12345789", 14)); n != 14 {
		t.Errorf("got %d", n)
	}
	if v := achFilenameSeqToStr(11); v != "B" {
		t.Errorf("got %s", v)
	}
}

func TestFileTransferController__ACHFile(t *testing.T) {
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

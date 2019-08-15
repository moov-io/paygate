// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/filetransfer"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestFileTransferController__newFileTransferController(t *testing.T) {
	dir, err := ioutil.TempDir("", "fileTransferController")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	repo := filetransfer.NewRepository(nil, "local") // filetransfer.localFileTransferRepository

	controller, err := newFileTransferController(log.NewNopLogger(), dir, repo, nil, nil, true)
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
	cutoff := &filetransfer.CutoffTime{
		RoutingNumber: "123",
		Cutoff:        1700,
		Loc:           time.UTC,
	}
	controller := &fileTransferController{
		ftpConfigs: []*filetransfer.FTPConfig{
			{
				RoutingNumber: "123",
				Hostname:      "ftp.foo.com",
			},
			{
				RoutingNumber: "321",
				Hostname:      "ftp.bar.com",
			},
		},
		fileTransferConfigs: []*filetransfer.Config{
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
	fileTransferConf = controller.findFileTransferConfig(&filetransfer.CutoffTime{RoutingNumber: "456"})
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
	controller.sftpConfigs = append(controller.sftpConfigs, &filetransfer.SFTPConfig{
		RoutingNumber: "987654320",
	})
	if v := controller.findTransferType("987654320"); v != "sftp" {
		t.Errorf("got %s", v)
	}

	// 'ftp' is checked first, so let's override that now
	controller.ftpConfigs = append(controller.ftpConfigs, &filetransfer.FTPConfig{
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

	repo := filetransfer.NewRepository(nil, "local") // filetransfer.localFileTransferRepository

	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	innerDepRepo := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}
	depRepo := &mockDepositoryRepository{
		cur: &microDepositCursor{
			batchSize: 5,
			depRepo:   innerDepRepo,
		},
	}
	transferRepo := &mockTransferRepository{
		cur: &transferCursor{
			batchSize:    5,
			transferRepo: &sqliteTransferRepo{db.DB, log.NewNopLogger()},
		},
	}

	// write a micro-deposit
	amt, _ := NewAmount("USD", "0.22")
	if err := innerDepRepo.initiateMicroDeposits(DepositoryID("depositoryID"), "userID", []microDeposit{{*amt, "fileID"}}); err != nil {
		t.Fatal(err)
	}

	achClient, _, achServer := achclient.MockClientServer("mergeGroupableTransfer", func(r *mux.Router) {
		achFileContentsRoute(r)
	})
	defer achServer.Close()

	// setuo transfer controller to start a manual merge and upload
	controller, err := newFileTransferController(logger, dir, repo, achClient, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	forceUpload := make(chan struct{}, 1)
	ctx, cancelFileSync := context.WithCancel(context.Background())

	go controller.startPeriodicFileOperations(ctx, forceUpload, depRepo, transferRepo) // async call to register the polling loop
	forceUpload <- struct{}{}                                                          // trigger the calls

	time.Sleep(250 * time.Millisecond)

	cancelFileSync()
}

func TestFileTransferController__writeFiles(t *testing.T) {
	dir, _ := ioutil.TempDir("", "file-transfer-async")
	defer os.RemoveAll(dir)

	controller := &fileTransferController{}
	files := []filetransfer.File{
		{
			Filename: "write-test",
			Contents: ioutil.NopCloser(strings.NewReader("test conents")),
		},
	}
	if err := controller.writeFiles(files, dir); err != nil {
		t.Error(err)
	}

	// verify file was written
	bs, err := ioutil.ReadFile(filepath.Join(dir, "write-test"))
	if err != nil {
		t.Error(err)
	}
	if v := string(bs); v != "test conents" {
		t.Errorf("got %q", v)
	}
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
	inboundFiles []filetransfer.File
	returnFiles  []filetransfer.File
	uploadedFile *filetransfer.File // non-nil on file upload
	deletedFile  string             // filepath of last deleted file
	mu           sync.RWMutex       // protects all fields
}

func (a *mockFileTransferAgent) GetInboundFiles() ([]filetransfer.File, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.inboundFiles, nil
}

func (a *mockFileTransferAgent) GetReturnFiles() ([]filetransfer.File, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.returnFiles, nil
}

func (a *mockFileTransferAgent) UploadFile(f filetransfer.File) error {
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

func TestFileTransferController__saveRemoteFiles(t *testing.T) {
	agent := &mockFileTransferAgent{
		inboundFiles: []filetransfer.File{
			{
				Filename: "ppd-debit.ach",
				Contents: readFileAsCloser(filepath.Join("testdata", "ppd-debit.ach")),
			},
		},
		returnFiles: []filetransfer.File{
			{
				Filename: "return-WEB.ach",
				Contents: readFileAsCloser(filepath.Join("testdata", "return-WEB.ach")),
			},
		},
	}
	dir, err := ioutil.TempDir("", "saveRemoteFiles")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	controller := &fileTransferController{
		rootDir: dir, // use our temp dir
		logger:  log.NewNopLogger(),
	}
	if err := controller.saveRemoteFiles(agent, dir); err != nil {
		t.Error(err)
	}

	// read written files
	file, err := parseACHFilepath(filepath.Join(dir, agent.InboundPath(), "ppd-debit.ach"))
	if err != nil {
		t.Error(err)
	}
	if v := file.Batches[0].GetHeader().StandardEntryClassCode; v != "PPD" {
		t.Errorf("SEC code found is %s", v)
	}
	file, err = parseACHFilepath(filepath.Join(dir, agent.ReturnPath(), "return-WEB.ach"))
	if err != nil {
		t.Error(err)
	}
	if v := file.Batches[0].GetHeader().StandardEntryClassCode; v != "WEB" {
		t.Errorf("SEC code found is %s", v)
	}

	// latest deleted file should be our return WEB
	if !strings.Contains(agent.deletedFile, "return-WEB.ach") && !strings.Contains(agent.deletedFile, "ppd-debit.ach") {
		t.Errorf("deleted file was %s", agent.deletedFile)
	}
}
func TestFileTransferController__filesNearTheirCutoff(t *testing.T) {
	nyc, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(nyc)

	dir, err := ioutil.TempDir("", "filesNearTheirCutoff")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// use a valid file
	src, err := os.Open(filepath.Join("testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.Create(filepath.Join(dir, achFilename("987654320", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}

	// Setup our cutoff time to be "just head" in time
	cutoffTimes := []*filetransfer.CutoffTime{
		{
			RoutingNumber: "987654320",
			Cutoff:        (now.Hour() * 100) + now.Minute() + 1, // 1 minute in the future in HHmm
			Loc:           nyc,
		},
	}

	outFiles, err := filesNearTheirCutoff(cutoffTimes, dir)
	if err != nil {
		t.Error(err)
	}
	if len(outFiles) != 1 {
		t.Fatalf("got %d files, expected one file for upload", len(outFiles))
	}
	if outFiles[0] == nil || outFiles[0].File == nil {
		t.Fatalf("outFiles[0]=%#v", outFiles[0])
	}

	// verify the file exists (for the sad path test)
	fds, _ := ioutil.ReadDir(dir)
	if len(fds) != 1 {
		t.Errorf("got %d files", len(fds))
	}

	// bump out time ahead
	cutoffTimes[0].Cutoff += 100 // add one hour
	outFiles, err = filesNearTheirCutoff(cutoffTimes, dir)
	if err != nil {
		t.Error(err)
	}
	if len(outFiles) != 0 {
		t.Fatal("expected no files (as cutoff is far enough forward in time)")
	}
}

func TestFileTransferController__mergeTransfer(t *testing.T) {
	// build a mergableFile from an example WEB entry
	webFile, err := parseACHFilepath(filepath.Join("testdata", "return-WEB.ach"))
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := ioutil.TempDir("", "mergeTransfer")
	defer os.RemoveAll(dir)
	mergableFile := &achFile{
		File:     ach.NewFile(),
		filepath: filepath.Join(dir, achFilename(webFile.Header.ImmediateDestination, 1)),
	}
	mergableFile.Header = ach.NewFileHeader()
	mergableFile.Header.ImmediateDestination = webFile.Header.ImmediateDestination
	mergableFile.Header.ImmediateOrigin = webFile.Header.ImmediateOrigin
	mergableFile.Header.FileCreationDate = time.Now().Format("060102")
	mergableFile.Header.ImmediateDestinationName = webFile.Header.ImmediateDestinationName
	mergableFile.Header.ImmediateOriginName = webFile.Header.ImmediateOriginName
	// Add 10000 batches to mergableFile (so it's over the LoC limit)
	for i := 0; i < 10000; i++ {
		mergableFile.AddBatch(webFile.Batches[0]) // AddBatch doesn't do unique-ness checks
	}
	if err := mergableFile.Create(); err != nil {
		t.Fatal(err)
	}

	file, err := parseACHFilepath(filepath.Join("testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	file.Header.ImmediateDestination = webFile.Header.ImmediateDestination
	file.Header.ImmediateOrigin = webFile.Header.ImmediateOrigin

	// call .mergeTransfer
	controller := &fileTransferController{
		logger: log.NewNopLogger(),
	}
	fileToUpload, err := controller.mergeTransfer(file, mergableFile)
	if err != nil {
		t.Fatal(err)
	}
	if v := filepath.Base(fileToUpload.filepath); v != fmt.Sprintf("%s-091400606-1.ach", time.Now().Format("20060102")) {
		t.Errorf("got %q", v)
	}

	// grab the latest mergable file and verify it's '*-2.ach'
	mergableFile, err = grabLatestMergedACHFile(webFile.Header.ImmediateDestination, file, dir)
	if err != nil {
		t.Fatal(err)
	}
	if v := filepath.Base(mergableFile.filepath); v != fmt.Sprintf("%s-091400606-2.ach", time.Now().Format("20060102")) {
		t.Errorf("got %q", v)
	}
}

var (
	achFileContentsRoute = func(r *mux.Router) {
		r.Methods("GET").Path("/files/{file_id}/contents").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")

			// read a test ACH file to write back
			fd, err := os.Open(filepath.Join("testdata", "ppd-debit.ach"))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf(`{"error": "%s"}`, err)))
				return
			}
			w.WriteHeader(http.StatusOK)
			io.Copy(w, fd)
			fd.Close()
		})
	}
)

func TestFileTransferController__mergeGroupableTransfer(t *testing.T) {
	achClient, _, achServer := achclient.MockClientServer("mergeGroupableTransfer", func(r *mux.Router) {
		achFileContentsRoute(r)
	})
	defer achServer.Close()

	dir, err := ioutil.TempDir("", "mergeGroupableTransfer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	controller := &fileTransferController{
		ach:    achClient,
		logger: log.NewNopLogger(),
	}

	xfer := &groupableTransfer{
		Transfer: &Transfer{
			ID: TransferID(base.ID()),
		},
		origin: "076401251", // from testdata/ppd-debit.ach
	}

	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &mockTransferRepository{}
	repo.fileID = "foo" // some non-empty value, our test ACH server doesn't care
	if fileToUpload := controller.mergeGroupableTransfer(dir, xfer, repo); fileToUpload != nil {
		t.Errorf("didn't expect fileToUpload=%v", fileToUpload)
	}

	// technically we load it twice, but we're reading the same file..
	file, err := controller.loadIncomingFile("foo")
	if err != nil {
		t.Fatal(err)
	}

	// check our mergable files
	mergableFile, err := grabLatestMergedACHFile(xfer.origin, file, dir)
	if err != nil {
		t.Fatal(err)
	}

	// verify the file exists
	if len(mergableFile.Batches) != 1 {
		t.Errorf("len(mergableFile.Batches)=%d", len(mergableFile.Batches))
	}
	if !mergableFile.Batches[0].Equal(file.Batches[0]) {
		t.Errorf("Batches aren't equal!")
	}
}

func TestFileTransferController__mergeMicroDeposit(t *testing.T) {
	achClient, _, achServer := achclient.MockClientServer("mergeMicroDeposit", func(r *mux.Router) {
		achFileContentsRoute(r)
	})
	defer achServer.Close()

	dir, err := ioutil.TempDir("", "mergeMicroDeposit")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	controller := &fileTransferController{
		ach:    achClient,
		logger: log.NewNopLogger(),
	}

	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	depRepo := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}

	// Setup our micro-deposit
	amt, _ := NewAmount("USD", "0.22")
	mc := uploadableMicroDeposit{
		depositoryID: "depositoryID",
		userID:       "userID",
		fileID:       "fileID",
		amount:       amt,
	}
	if err := depRepo.initiateMicroDeposits(DepositoryID("depositoryID"), "userID", []microDeposit{{*amt, "fileID"}}); err != nil {
		t.Fatal(err)
	}

	// write a Depository
	if err := depRepo.upsertUserDepository("userID", &Depository{
		ID:            "depositoryID",
		BankName:      "Mooc, Inc",
		RoutingNumber: "987654320",
	}); err != nil {
		t.Fatal(err)
	}

	if fileToUpload := controller.mergeMicroDeposit(dir, mc, depRepo); fileToUpload != nil {
		t.Errorf("didn't expect an ACH file to upload: %#v", fileToUpload)
	}

	mergedFilename, err := readMergedFilename(depRepo, amt, DepositoryID(mc.depositoryID))
	if err != nil {
		t.Fatal(err)
	}
	if v := fmt.Sprintf("%s-987654320-1.ach", time.Now().Format("20060102")); mergedFilename != v {
		t.Errorf("got mergedFilename=%s", v)
	}
}

func TestFileTransferController__startUploadError(t *testing.T) {
	nyc, _ := time.LoadLocation("America/New_York")
	controller := &fileTransferController{
		cutoffTimes: []*filetransfer.CutoffTime{
			{
				RoutingNumber: "987654320",
				Cutoff:        1700,
				Loc:           nyc,
			},
		},
		fileTransferConfigs: []*filetransfer.Config{
			{
				RoutingNumber: "987654320",
				OutboundPath:  "outbound/",
			},
		},
		logger: log.NewNopLogger(),
	}

	// Setup our test file for upload
	file := ach.NewFile()
	file.Header = ach.NewFileHeader()
	file.Header.ImmediateOrigin = "987654320"

	var filesToUpload = []*achFile{
		{File: file, filepath: "/dev/null"}, // invalid filepath
	}

	if err := controller.startUpload(filesToUpload); err == nil {
		t.Error("expected error")
	}
}

func TestFileTransferController__uploadFile(t *testing.T) {
	agent := &mockFileTransferAgent{}
	file, err := parseACHFilepath(filepath.Join("testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	controller := &fileTransferController{
		logger: log.NewNopLogger(),
	}
	if err := controller.uploadFile(agent, &achFile{File: file, filepath: filepath.Join("testdata", "ppd-debit.ach")}); err != nil {
		t.Error(err)
	}

	if agent.uploadedFile == nil {
		t.Fatal("nil agent.uploadedFile")
	}
	if v := agent.uploadedFile.Filename; v != "ppd-debit.ach" {
		t.Errorf("got %v", v)
	}
	if bs, err := ioutil.ReadAll(agent.uploadedFile.Contents); len(bs) == 0 || err != nil {
		t.Errorf("copied empty file: %v", err)
	}
}

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
	file, err := parseACHFilepath(filepath.Join("testdata", "ppd-debit.ach"))
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
	file, err := parseACHFilepath(filepath.Join("testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	af := &achFile{File: file}
	af.removeBatch(file.Batches[0])
	if len(af.Batches) != 0 {
		t.Errorf("got %d batches", len(af.Batches))
	}
}

func TestFileTransferController__groupTransfers(t *testing.T) {
	transfers := []*groupableTransfer{
		{
			Transfer: &Transfer{
				ID: "1",
			},
			origin: "123456789",
		},
		{
			Transfer: &Transfer{
				ID: "2",
			},
			origin: "123456789",
		},
		{
			Transfer: &Transfer{
				ID: "3",
			},
			origin: "987654321",
		},
	}
	grouped, err := groupTransfers(transfers, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grouped) != 2 {
		t.Fatalf("len(grouped)=%d", len(grouped))
	}
	first, second := grouped[0], grouped[1]
	if first[0].ID != "1" && first[1].ID != "2" {
		t.Errorf("first[0].ID=%s first[1].ID=%s", first[0].ID, first[1].ID)
	}
	if second[0].ID != "3" {
		t.Errorf("second[0].ID=%s", second[0].ID)
	}

	// ensure we error if err != nil
	grouped, err = groupTransfers(transfers, errors.New("test error"))
	if err == nil || len(grouped) != 0 {
		t.Errorf("expected error but got none, len(grouped)=%d", len(grouped))
	}
}

func TestFileTransferController__grabLatestMergedACHFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "grabLatestMergedACHFile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	routingNumber := "123456789"

	// write two files under achFilename (same routingNumber, diff seq)
	if err := writeACHFile(filepath.Join(dir, achFilename(routingNumber, 1))); err != nil {
		t.Fatal(err)
	}
	if err := writeACHFile(filepath.Join(dir, achFilename(routingNumber, 2))); err != nil {
		t.Fatal(err)
	}
	file, err := grabLatestMergedACHFile(routingNumber, nil, dir) // don't need an achFile
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("nil achFile")
	}
	if file.filepath != filepath.Join(dir, achFilename(routingNumber, 2)) {
		t.Errorf("got %q expected %q", file.filepath, filepath.Join(dir, achFilename(routingNumber, 2)))
	}

	// Then look for a new ABA and ensure we get a new achFile created
	incoming, err := parseACHFilepath(filepath.Join("testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	incoming.Header.ImmediateOrigin = "432156784" // random routing number
	incoming.Header.ImmediateOriginName = "origin bank"
	incoming.Header.ImmediateDestination = "987654320"
	incoming.Header.ImmediateDestinationName = "destination bank"
	incoming.Header.FileCreationDate = time.Now().Format("060102") // YYMMDD
	incoming.Header.FileCreationTime = time.Now().Format("1504")   // HHMM
	if err := incoming.Create(); err != nil {
		t.Fatal(err)
	}

	file, err = grabLatestMergedACHFile(incoming.Header.ImmediateDestination, incoming, dir)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("nil achFile")
	}
	if name := achFilename("987654320", 1); file.filepath != filepath.Join(dir, name) {
		t.Errorf("got %q expected %q", file.filepath, filepath.Join(dir, name))
	}
}

func writeACHFile(path string) error {
	fd, err := os.Open(filepath.Join("testdata", "ppd-debit.ach"))
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

func TestFileTransferController__processReturnEntry(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("testdata", "return-WEB.ach"))
	if err != nil {
		t.Fatal(err)
	}
	b := file.Batches[0]

	// Force the ReturnCode to a value we want for our tests
	b.GetEntries()[0].Addenda99.ReturnCode = "R02" // "Account Closed"

	amt, _ := NewAmount("USD", "52.12")
	userID, transactionID := base.ID(), base.ID()

	depRepo := &mockDepositoryRepository{
		depositories: []*Depository{
			{
				ID:            DepositoryID(base.ID()), // Don't use either DepositoryID from below
				BankName:      "my bank",
				Holder:        "jane doe",
				HolderType:    Individual,
				Type:          Savings,
				RoutingNumber: file.Header.ImmediateOrigin,
				AccountNumber: "123121",
				Status:        DepositoryVerified,
				Metadata:      "other info",
			},
			{
				ID:            DepositoryID(base.ID()), // Don't use either DepositoryID from below
				BankName:      "their bank",
				Holder:        "john doe",
				HolderType:    Individual,
				Type:          Savings,
				RoutingNumber: file.Header.ImmediateDestination,
				AccountNumber: b.GetEntries()[0].DFIAccountNumber,
				Status:        DepositoryVerified,
				Metadata:      "other info",
			},
		},
	}
	transferRepo := &mockTransferRepository{
		xfer: &Transfer{
			Type:                   PushTransfer,
			Amount:                 *amt,
			Originator:             OriginatorID("originator"),
			OriginatorDepository:   DepositoryID("orig-depository"),
			Receiver:               ReceiverID("receiver"),
			ReceiverDepository:     DepositoryID("rec-depository"),
			Description:            "transfer",
			StandardEntryClassCode: "PPD",
			userID:                 userID,
			transactionID:          transactionID,
		},
	}

	dir, _ := ioutil.TempDir("", "processReturnEntry")
	defer os.RemoveAll(dir)

	repo := filetransfer.NewRepository(nil, "local") // filetransfer.localFileTransferRepository

	controller, err := newFileTransferController(log.NewNopLogger(), dir, repo, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0], depRepo, transferRepo); err != nil {
		t.Error(err)
	}

	// Check for our updated statuses
	if depRepo.status != DepositoryRejected {
		t.Errorf("Depository status wasn't updated, got %v", depRepo.status)
	}
	if transferRepo.returnCode != "R02" {
		t.Errorf("unexpected return code: %s", transferRepo.returnCode)
	}
	if transferRepo.status != TransferReclaimed {
		t.Errorf("unexpected status: %v", transferRepo.status)
	}

	// Check quick error conditions
	depRepo.err = errors.New("bad error")
	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0], depRepo, transferRepo); err == nil {
		t.Error("expected error")
	}
	depRepo.err = nil

	transferRepo.err = errors.New("bad error")
	if err := controller.processReturnEntry(file.Header, b.GetHeader(), b.GetEntries()[0], depRepo, transferRepo); err == nil {
		t.Error("expected error")
	}
	transferRepo.err = nil
}

// depositoryReturnCode writes two Depository objects into a database and then calls updateTransferFromReturnCode
// over the provided return code. The two Depository objects returned are re-read from the database after.
func depositoryReturnCode(t *testing.T, code string) (*Depository, *Depository) {
	t.Helper()

	logger := log.NewNopLogger()

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	repo := &sqliteDepositoryRepo{sqliteDB.DB, logger}

	userID := base.ID()
	origDep := &Depository{
		ID:       DepositoryID(base.ID()),
		BankName: "originator bank",
		Status:   DepositoryVerified,
	}
	if err := repo.upsertUserDepository(userID, origDep); err != nil {
		t.Fatal(err)
	}
	recDep := &Depository{
		ID:       DepositoryID(base.ID()),
		BankName: "receiver bank",
		Status:   DepositoryVerified,
	}
	if err := repo.upsertUserDepository(userID, recDep); err != nil {
		t.Fatal(err)
	}

	rc := &ach.ReturnCode{Code: code}
	if err := updateTransferFromReturnCode(logger, rc, origDep, recDep, repo); err != nil {
		t.Fatal(err)
	}

	// re-read and return the Depository objects
	oDep, _ := repo.getUserDepository(origDep.ID, userID)
	rDep, _ := repo.getUserDepository(recDep.ID, userID)
	return oDep, rDep
}

func TestFiles__updateTransferFromReturnCode(t *testing.T) {
	// R02, R07, R10
	if orig, rec := depositoryReturnCode(t, "R02"); orig.Status != DepositoryVerified || rec.Status != DepositoryRejected {
		t.Errorf("orig.Status=%s rec.Status=%s", orig.Status, rec.Status)
	}
	if orig, rec := depositoryReturnCode(t, "R07"); orig.Status != DepositoryVerified || rec.Status != DepositoryRejected {
		t.Errorf("orig.Status=%s rec.Status=%s", orig.Status, rec.Status)
	}
	if orig, rec := depositoryReturnCode(t, "R10"); orig.Status != DepositoryVerified || rec.Status != DepositoryRejected {
		t.Errorf("orig.Status=%s rec.Status=%s", orig.Status, rec.Status)
	}

	// R05
	if orig, rec := depositoryReturnCode(t, "R05"); orig.Status != DepositoryVerified || rec.Status != DepositoryRejected {
		t.Errorf("orig.Status=%s rec.Status=%s", orig.Status, rec.Status)
	}

	// R14, R15
	if orig, rec := depositoryReturnCode(t, "R14"); orig.Status != DepositoryRejected || rec.Status != DepositoryRejected {
		t.Errorf("orig.Status=%s rec.Status=%s", orig.Status, rec.Status)
	}
	if orig, rec := depositoryReturnCode(t, "R15"); orig.Status != DepositoryRejected || rec.Status != DepositoryRejected {
		t.Errorf("orig.Status=%s rec.Status=%s", orig.Status, rec.Status)
	}

	// R16
	if orig, rec := depositoryReturnCode(t, "R16"); orig.Status != DepositoryVerified || rec.Status != DepositoryRejected {
		t.Errorf("orig.Status=%s rec.Status=%s", orig.Status, rec.Status)
	}

	// R20
	if orig, rec := depositoryReturnCode(t, "R20"); orig.Status != DepositoryVerified || rec.Status != DepositoryRejected {
		t.Errorf("orig.Status=%s rec.Status=%s", orig.Status, rec.Status)
	}
}

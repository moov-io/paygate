// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
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

	repo := &localFileTransferRepository{}
	controller, err := newFileTransferController(log.NewNopLogger(), dir, repo)
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
	if len(controller.sftpConfigs) != 1 {
		t.Errorf("local len(controller.sftpConfigs)=%d", len(controller.sftpConfigs))
	}
	if len(controller.fileTransferConfigs) != 1 {
		t.Errorf("local len(controller.fileTransferConfigs)=%d", len(controller.fileTransferConfigs))
	}
}

func TestFileTransferController__getDetails(t *testing.T) {
	cutoff := &cutoffTime{
		routingNumber: "123",
		cutoff:        1700,
		loc:           time.UTC,
	}
	controller := &fileTransferController{
		sftpConfigs: []*sftpConfig{
			{
				RoutingNumber: "123",
				Hostname:      "sftp.foo.com",
			},
			{
				RoutingNumber: "321",
				Hostname:      "sftp.bar.com",
			},
		},
		fileTransferConfigs: []*fileTransferConfig{
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
	sftpConf, fileTransferConf := controller.getDetails(cutoff)
	if sftpConf == nil || fileTransferConf == nil {
		t.Fatalf("sftpConf=%v fileTransferConf=%v", sftpConf, fileTransferConf)
	}
	if sftpConf.Hostname != "sftp.foo.com" {
		t.Errorf("sftpConf=%#v", sftpConf)
	}
	if fileTransferConf.InboundPath != "inbound/" {
		t.Errorf("fileTransferConf=%#v", fileTransferConf)
	}

	// not found
	sftpConf, fileTransferConf = controller.getDetails(&cutoffTime{routingNumber: "456"})
	if sftpConf != nil || fileTransferConf != nil {
		t.Fatalf("sftpConf=%v fileTransferConf=%v", sftpConf, fileTransferConf)
	}
}

func TestFileTransferController__writeFiles(t *testing.T) {
	dir, _ := ioutil.TempDir("", "file-transfer-async")
	defer os.RemoveAll(dir)

	controller := &fileTransferController{}
	files := []file{
		{
			filename: "write-test",
			contents: ioutil.NopCloser(strings.NewReader("test conents")),
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

func TestFileTransferController__saveRemoteFiles(t *testing.T) {
	agent := &mockFileTransferAgent{
		inboundFiles: []file{
			{
				filename: "ppd-debit.ach",
				contents: readFileAsCloser(filepath.Join("testdata", "ppd-debit.ach")),
			},
		},
		returnFiles: []file{
			{
				filename: "return-WEB.ach",
				contents: readFileAsCloser(filepath.Join("testdata", "return-WEB.ach")),
			},
		},
	}
	dir, err := ioutil.TempDir("", "saveRemoteFiles")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	controller := &fileTransferController{
		logger: log.NewNopLogger(),
	}
	if err := controller.saveRemoteFiles(agent, dir); err != nil {
		t.Error(err)
	}

	// read written files
	file, err := parseACHFilepath(filepath.Join(dir, agent.inboundPath(), "ppd-debit.ach"))
	if err != nil {
		t.Error(err)
	}
	if v := file.Batches[0].GetHeader().StandardEntryClassCode; v != "PPD" {
		t.Errorf("SEC code found is %s", v)
	}
	file, err = parseACHFilepath(filepath.Join(dir, agent.returnPath(), "return-WEB.ach"))
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
		mergableFile.AddBatch(webFile.Batches[0])
	}
	mergableFile.Create()

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

// func (c *fileTransferController) mergeGroupableTransfer(mergedDir string, xfer *groupableTransfer, transferRepo transferRepository) *achFile {

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
		destination: "076401251", // from testdata/ppd-debit.ach
	}

	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	repo := &mockTransferRepository{}
	repo.fileId = "foo" // some non-empty value, our test ACH server doesn't care
	if fileToUpload := controller.mergeGroupableTransfer(dir, xfer, repo); fileToUpload != nil {
		t.Errorf("didn't expect fileToUpload=%v", fileToUpload)
	}

	// technically we load it twice, but we're reading the same file..
	file, err := controller.loadIncomingFile(xfer, repo)
	if err != nil {
		t.Fatal(err)
	}

	// check our mergable files
	mergableFile, err := grabLatestMergedACHFile(xfer.destination, file, dir) // TODO(adam): can file be nil here?
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
	if v := agent.uploadedFile.filename; v != "ppd-debit.ach" {
		t.Errorf("got %v", v)
	}

	if bs, err := ioutil.ReadAll(agent.uploadedFile.contents); len(bs) == 0 || err != nil {
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
			destination: "123456789",
		},
		{
			Transfer: &Transfer{
				ID: "2",
			},
			destination: "123456789",
		},
		{
			Transfer: &Transfer{
				ID: "3",
			},
			destination: "987654321",
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
	incoming.Header.ImmediateOrigin = "432156789" // random routing number
	incoming.Header.ImmediateOriginName = "origin bank"
	incoming.Header.ImmediateDestination = "987654321"
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
	if file.filepath != filepath.Join(dir, achFilename("987654321", 1)) {
		t.Errorf("got %q expected %q", file.filepath, filepath.Join(dir, achFilename("987654321", 1)))
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

type testSqliteFileTransferRepository struct {
	*sqliteFileTransferRepository

	testDB *testSqliteDB
}

func (r *testSqliteFileTransferRepository) close() error {
	r.sqliteFileTransferRepository.close()
	return r.testDB.close()
}

func createTestSqliteFileTransferRepository(t *testing.T) *testSqliteFileTransferRepository {
	t.Helper()

	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	repo := &sqliteFileTransferRepository{db: db.db}
	return &testSqliteFileTransferRepository{repo, db}
}

func TestSqliteFileTransferRepository__getCounts(t *testing.T) {
	repo := createTestSqliteFileTransferRepository(t)
	defer repo.close()

	writeCutoffTime(t, repo)
	writeSFTPConfig(t, repo)
	writeFileTransferConfig(t, repo)

	cutoffs, sftps, filexfers := repo.getCounts()
	if cutoffs != 1 {
		t.Errorf("got %d", cutoffs)
	}
	if sftps != 1 {
		t.Errorf("got %d", sftps)
	}
	if filexfers != 1 {
		t.Errorf("got %d", filexfers)
	}

	// If we read at least one row from each config table we need to make sure newFileTransferRepository
	// returns sqliteFileTransferRepository (rather than localFileTransferRepository)
	r := newFileTransferRepository(repo.db)
	if _, ok := r.(*sqliteFileTransferRepository); !ok {
		t.Errorf("got %T", r)
	}
}

func writeCutoffTime(t *testing.T, repo *testSqliteFileTransferRepository) {
	t.Helper()

	query := `insert into cutoff_times (routing_number, cutoff, location) values ('123456789', 1700, 'America/New_York');`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func TestSqliteFileTransferRepository__getCutoffTimes(t *testing.T) {
	repo := createTestSqliteFileTransferRepository(t)
	defer repo.close()

	writeCutoffTime(t, repo)

	cutoffTimes, err := repo.getCutoffTimes()
	if err != nil {
		t.Fatal(err)
	}
	if len(cutoffTimes) != 1 {
		t.Errorf("len(cutoffTimes)=%d", len(cutoffTimes))
	}
	if cutoffTimes[0].routingNumber != "123456789" {
		t.Errorf("cutoffTimes[0].routingNumber=%s", cutoffTimes[0].routingNumber)
	}
	if cutoffTimes[0].cutoff != 1700 {
		t.Errorf("cutoffTimes[0].cutoff=%d", cutoffTimes[0].cutoff)
	}
	if v := cutoffTimes[0].loc.String(); v != "America/New_York" {
		t.Errorf("cutoffTimes[0].loc=%v", v)
	}
}

func writeSFTPConfig(t *testing.T, repo *testSqliteFileTransferRepository) {
	t.Helper()

	query := `insert into sftp_configs (routing_number, hostname, username, password) values ('123456789', 'ftp.moov.io', 'moov', 'secret');`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func TestSqliteFileTransferRepository__getSFTPConfigs(t *testing.T) {
	repo := createTestSqliteFileTransferRepository(t)
	defer repo.close()

	writeSFTPConfig(t, repo)

	// now read
	configs, err := repo.getSFTPConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Errorf("len(configs)=%d", len(configs))
	}
	if configs[0].RoutingNumber != "123456789" {
		t.Errorf("got %q", configs[0].RoutingNumber)
	}
	if configs[0].Hostname != "ftp.moov.io" {
		t.Errorf("got %q", configs[0].Hostname)
	}
	if configs[0].Username != "moov" {
		t.Errorf("got %q", configs[0].Username)
	}
	if configs[0].Password != "secret" {
		t.Errorf("got %q", configs[0].Password)
	}
}

func writeFileTransferConfig(t *testing.T, repo *testSqliteFileTransferRepository) {
	t.Helper()

	query := `insert into file_transfer_configs (routing_number, inbound_path, outbound_path, return_path) values ('123456789', 'inbound/', 'outbound/', 'return/');`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func TestSqliteFileTransferRepository__getFileTransferConfigs(t *testing.T) {
	repo := createTestSqliteFileTransferRepository(t)
	defer repo.close()

	writeFileTransferConfig(t, repo)

	// now read
	configs, err := repo.getFileTransferConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Errorf("len(configs)=%d", len(configs))
	}
	if configs[0].RoutingNumber != "123456789" {
		t.Errorf("got %q", configs[0].RoutingNumber)
	}
	if configs[0].InboundPath != "inbound/" {
		t.Errorf("got %q", configs[0].InboundPath)
	}
	if configs[0].OutboundPath != "outbound/" {
		t.Errorf("got %q", configs[0].OutboundPath)
	}
	if configs[0].ReturnPath != "return/" {
		t.Errorf("got %q", configs[0].ReturnPath)
	}
}

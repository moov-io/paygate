// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestFileTransferController__writeFile(t *testing.T) {
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

func TestFileTransferController__achFilename(t *testing.T) {
	now := time.Now().Format("20060102")

	if v := achFilename("12345789", 2); v != fmt.Sprintf("%s-12345789-2.ach", now) {
		t.Errorf("got %q", v)
	}
}

func TestFileTransferController__ACHFile(t *testing.T) {
	fd, err := os.Open(filepath.Join("testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	file, err := parseACHFile(fd)
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
	file, err := grabLatestMergedACHFile(routingNumber, dir)
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
	file, err = grabLatestMergedACHFile("987654321", dir)
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

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
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

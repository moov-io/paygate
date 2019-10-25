// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moov-io/paygate/internal/config"
)

func TestController__writeFiles(t *testing.T) {
	dir, _ := ioutil.TempDir("", "file-transfer-async")
	defer os.RemoveAll(dir)

	controller := &Controller{}
	files := []File{
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

func TestController__saveRemoteFiles(t *testing.T) {
	agent := &mockFileTransferAgent{
		inboundFiles: []File{
			{
				Filename: "ppd-debit.ach",
				Contents: readFileAsCloser(filepath.Join("..", "..", "testdata", "ppd-debit.ach")),
			},
		},
		returnFiles: []File{
			{
				Filename: "return-WEB.ach",
				Contents: readFileAsCloser(filepath.Join("..", "..", "testdata", "return-WEB.ach")),
			},
		},
	}
	dir, err := ioutil.TempDir("", "saveRemoteFiles")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	controller := &Controller{
		rootDir: dir, // use our temp dir
		cfg:     config.Empty(),
	}
	controller.cfg.ACH.StorageDir = dir
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

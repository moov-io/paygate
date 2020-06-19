// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/moov-io/paygate/pkg/upload"

	"github.com/go-kit/kit/log"
)

func TestCleanupErr(t *testing.T) {
	agent := &upload.MockAgent{
		Err: errors.New("bad error"),
	}

	dir, _ := ioutil.TempDir("", "clenaup-testing")
	dl := &downloadedFiles{dir: dir}
	defer dl.deleteFiles()

	// write a test file to attempt deletion
	path := filepath.Join(dl.dir, agent.InboundPath())
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(path, "file.ach"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// test out cleanup func
	if err := Cleanup(log.NewNopLogger(), agent, dl); err == nil {
		t.Error("expected error")
	}

	if agent.DeletedFile != "inbound/file.ach" {
		t.Errorf("unexpected deleted file: %s", agent.DeletedFile)
	}
}

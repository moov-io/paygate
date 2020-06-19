// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/paygate/pkg/upload"
)

func TestDownloader__deleteFiles(t *testing.T) {
	factory := &downloaderImpl{
		logger:  log.NewNopLogger(),
		baseDir: testDir(t),
	}

	agent := &upload.MockAgent{}
	dl, err := factory.setup(agent)
	if err != nil {
		t.Fatal(err)
	}

	// write a file and expect it to be deleted
	path := filepath.Join(dl.dir, agent.InboundPath(), "foo.ach")
	if err := ioutil.WriteFile(path, []byte("testing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := dl.deleteFiles(); err != nil {
		t.Fatal(err)
	}

	// read files
	fds, err := ioutil.ReadDir(dl.dir)
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(fds) != 0 {
		t.Errorf("%d unexpected files", len(fds))
	}
}

func TestDownloader__deleteEmptyDirs(t *testing.T) {
	factory := &downloaderImpl{
		logger:  log.NewNopLogger(),
		baseDir: testDir(t),
	}

	agent := &upload.MockAgent{}
	dl, err := factory.setup(agent)
	if err != nil {
		t.Fatal(err)
	}

	// write a file and expect it to be deleted
	path := filepath.Join(dl.dir, agent.InboundPath(), "foo.ach")
	if err := ioutil.WriteFile(path, []byte("testing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := dl.deleteEmptyDirs(agent); err != nil {
		t.Fatal(err)
	}

	// read files
	fds, err := ioutil.ReadDir(dl.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fds) != 1 {
		t.Fatalf("%d unexpected files", len(fds))
	}
	if n := fds[0].Name(); n != "inbound" {
		t.Errorf("unexpected %v", n)
	}
	// Check the file still exists
	if _, err := os.Stat(path); err != nil {
		t.Error(err)
	}
}

func testDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "downloader")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

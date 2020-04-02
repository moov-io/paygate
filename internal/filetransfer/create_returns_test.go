// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moov-io/ach"
)

func TestController__uploadReturnFile(t *testing.T) {
	controller := setupTestController(t)

	out, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "cor-c01.ach"))
	if err != nil {
		t.Fatal(err)
	}

	if err := controller.uploadReturnFiles([]*ach.File{out}); err != nil {
		switch {
		case strings.Contains(err.Error(), "connect: connection refused"):
			// do nothing
		case strings.Contains(err.Error(), "No connection could be made"):
			// do nothing
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	fds, err := ioutil.ReadDir(controller.scratchDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(fds) != 1 {
		t.Errorf("got %d files", len(fds))
	}

	file, err := parseACHFilepath(filepath.Join(controller.scratchDir(), fds[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Error("got no file")
	}
}

func TestController__returnEntireFile(t *testing.T) {
	in, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "two-micro-deposits.ach"))
	if err != nil {
		t.Fatal(err)
	}

	outFiles, err := returnEntireFile(in, "R01")
	if err != nil {
		t.Fatal(err)
	}
	if len(outFiles) != 1 {
		t.Fatalf("got %d unexpected files", len(outFiles))
	}

	out := outFiles[0]
	if len(out.Batches) != 1 {
		t.Errorf("got %d batches", len(out.Batches))
	}

	entries := outFiles[0].Batches[0].GetEntries()
	if len(entries) != 6 {
		t.Errorf("got %d entries", len(entries))
	}
}

func TestController__returnEntry(t *testing.T) {
	in, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "prenote-ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Batches) != 1 {
		t.Fatalf("batches=%#v", in.Batches)
	}

	entry := in.Batches[0].GetEntries()[0]
	out, err := returnEntry(in.Header, in.Batches[0], entry, "R01")
	if err != nil {
		t.Fatal(err)
	}

	if len(out.Batches) != 1 {
		t.Fatalf("batches=%#v", out.Batches)
	}
	entries := out.Batches[0].GetEntries()
	if len(entries) != 1 {
		t.Fatalf("entries=%#v", entries)
	}

	if ok, err := isPrenoteEntry(entries[0]); !ok || err != nil {
		t.Errorf("expected prenote entry: %#v", entries[0])
		t.Error(err)
	}
}

func TestController__returnEntryErr(t *testing.T) {
	var fh ach.FileHeader
	if _, err := returnEntry(fh, nil, nil, "invalid"); err == nil {
		t.Error("expected error")
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"path/filepath"
	"testing"
)

func TestPrenote__returnEntry(t *testing.T) {
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

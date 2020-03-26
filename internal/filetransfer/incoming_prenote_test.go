// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"path/filepath"
	"testing"
)

func TestPrenote__isPrenoteEntry(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "prenote-ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	entries := file.Batches[0].GetEntries()
	if len(entries) != 1 {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	for i := range entries {
		if ok, err := isPrenoteEntry(entries[i]); !ok || err != nil {
			t.Errorf("expected prenote entry: %#v", entries[i])
			t.Error(err)
		}
	}

	// non prenote file
	file, err = parseACHFilepath(filepath.Join("..", "..", "testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	entries = file.Batches[0].GetEntries()
	for i := range entries {
		if ok, err := isPrenoteEntry(entries[i]); ok || err != nil {
			t.Errorf("expected no prenote entry: %#v", entries[i])
			t.Error(err)
		}
	}
}

func TestPrenote__isPrenoteEntryErr(t *testing.T) {
	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "prenote-ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	entries := file.Batches[0].GetEntries()
	if len(entries) != 1 {
		t.Fatalf("unexpected entries: %#v", entries)
	}

	entries[0].Amount = 125 // non-zero amount
	if exists, err := isPrenoteEntry(entries[0]); !exists || err == nil {
		t.Errorf("expected invalid prenote: %v", err)
	}
}

func TestPrenote__returnPrenoteEntry(t *testing.T) {
	in, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "prenote-ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Batches) != 1 {
		t.Fatalf("batches=%#v", in.Batches)
	}

	entry := in.Batches[0].GetEntries()[0]
	out, err := returnPrenoteEntry(in.Header, in.Batches[0], entry)
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

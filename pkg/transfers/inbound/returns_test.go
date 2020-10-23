// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/base/log"

	"github.com/moov-io/paygate/pkg/transfers"
)

func TestReturns__SetReturnCode(t *testing.T) {
	repo := &transfers.MockRepository{}
	ed := &ach.EntryDetail{
		Addenda99: ach.NewAddenda99(),
	}
	ed.Addenda99.ReturnCode = "R01"
	transferID := base.ID()

	if err := SaveReturnCode(repo, transferID, ed); err != nil {
		t.Error(err)
	}

	repo.Err = errors.New("bad error")
	if err := SaveReturnCode(repo, transferID, ed); err == nil {
		t.Error("expected error")
	}

	// missing values
	if err := SaveReturnCode(repo, transferID, nil); err == nil {
		t.Error("expected error")
	}
	if err := SaveReturnCode(repo, transferID, &ach.EntryDetail{}); err == nil {
		t.Error("expected error")
	}
}

func TestReturns__Handle(t *testing.T) {
	file, _ := ach.ReadFile(filepath.Join("testdata", "bh-ed-ad-bh-ed-ad-ed-ad.ach"))
	if len(file.Batches) != 1 {
		t.Fatalf("batches: %#v", file.Batches)
	}

	repo := &transfers.MockRepository{}
	processor := NewReturnProcessor(log.NewNopLogger(), repo)

	if err := processor.Handle(file); err != nil {
		t.Fatal(err)
	}

	// test with error from the repository
	repo.Err = errors.New("bad error")
	if err := processor.Handle(file); err == nil {
		t.Fatal("expected error")
	}
}

func TestReturns__processReturnEntry(t *testing.T) {
	file, _ := ach.ReadFile(filepath.Join("testdata", "bh-ed-ad-bh-ed-ad-ed-ad.ach"))
	if len(file.Batches) != 1 {
		t.Fatalf("batches: %#v", file.Batches)
	}

	fh := ach.NewFileHeader()
	bh := file.Batches[0].GetHeader()
	entry := file.Batches[0].GetEntries()[0]

	repo := &transfers.MockRepository{}
	processor := NewReturnProcessor(log.NewNopLogger(), repo)

	if err := processor.processReturnEntry(fh, bh, entry); err != nil {
		t.Fatal(err)
	}

	// test with error from the repository
	repo.Err = errors.New("bad error")
	if err := processor.processReturnEntry(fh, bh, entry); err == nil {
		t.Fatal("expected error")
	}
}

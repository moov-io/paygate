// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"errors"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
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

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/config"
)

func TestEmail__marshal(t *testing.T) {
	cfg := &config.Email{
		CompanyName: "Moov",
	}
	msg := &Message{
		Direction: Upload,
		Filename:  "20200529-131400.ach",
	}
	if f, err := ach.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "ppd-debit.ach")); err != nil {
		t.Fatal(err)
	} else {
		msg.File = f
	}

	contents, err := marshalEmail(cfg, msg)
	if err != nil {
		t.Fatal(err)
	}

	if testing.Verbose() {
		t.Log(contents)
	}

	if !strings.Contains(contents, `A file has been uploaded from Moov: 20200529-131400.ach`) {
		t.Error("generated template doesn't match")
	}
	if !strings.Contains(contents, `Debits:  10500`) {
		t.Error("generated template doesn't match")
	}
	if !strings.Contains(contents, `Credits: 0`) {
		t.Error("generated template doesn't match")
	}
	if !strings.Contains(contents, `Batches: 1`) {
		t.Error("generated template doesn't match")
	}
	if !strings.Contains(contents, `Total Entries: 1`) {
		t.Error("generated template doesn't match")
	}
}

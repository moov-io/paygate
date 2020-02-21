// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"encoding/json"
	"testing"
)

func TestAccountType(t *testing.T) {
	var tpe AccountType
	if !tpe.empty() {
		t.Error("expected empty")
	}
}

func TestAccountType__json(t *testing.T) {
	at := Checking

	// marshal
	bs, err := json.Marshal(&at)
	if err != nil {
		t.Fatal(err.Error())
	}
	if s := string(bs); s != `"checking"` {
		t.Errorf("got %q", s)
	}

	// unmarshal
	raw := []byte(`"Savings"`) // test other case
	if err := json.Unmarshal(raw, &at); err != nil {
		t.Error(err.Error())
	}
	if at != Savings {
		t.Errorf("got %s", at)
	}

	// expect failures
	raw = []byte("bad")
	if err := json.Unmarshal(raw, &at); err == nil {
		t.Error("expected error")
	}
}

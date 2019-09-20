// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package util

import (
	"testing"
)

func TestOr(t *testing.T) {
	if v := Or("", "backup"); v != "backup" {
		t.Errorf("v=%s", v)
	}
	if v := Or("primary", ""); v != "primary" {
		t.Errorf("v=%s", v)
	}
	if v := Or("primary", "backup"); v != "primary" {
		t.Errorf("v=%s", v)
	}
}

func TestYes(t *testing.T) {
	if !Yes("yes") {
		t.Error("expected true")
	}
	if Yes("no") {
		t.Error("expected no")
	}
	if Yes("") {
		t.Error("expected no")
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package util

import (
	"testing"
	"time"
)

func TestFirstParsedTime(t *testing.T) {
	tt := FirstParsedTime("2020-04-07")
	if !tt.IsZero() {
		t.Errorf("expected zero, got %v", tt)
	}

	tt = FirstParsedTime("2020-04-07", YYMMDDTimeFormat)
	if v := tt.Format(YYMMDDTimeFormat); v != "2020-04-07" {
		t.Errorf("got %v", v)
	}

	tt = FirstParsedTime(time.Now().Format(time.RFC3339), YYMMDDTimeFormat)
	if !tt.IsZero() {
		t.Errorf("expected zero, got %v", tt)
	}
}

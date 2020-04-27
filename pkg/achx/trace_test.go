// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"testing"
)

func TestTrace__ABA(t *testing.T) {
	routingNumber := "231380104"
	if v := ABA8(routingNumber); v != "23138010" {
		t.Errorf("got %s", v)
	}
	if v := ABACheckDigit(routingNumber); v != "4" {
		t.Errorf("got %s", v)
	}

	// 10 digit from ACH server
	if v := ABA8("0123456789"); v != "12345678" {
		t.Errorf("got %s", v)
	}
	if v := ABACheckDigit("0123456789"); v != "9" {
		t.Errorf("got %s", v)
	}
}

func TestTraceNumber(t *testing.T) {
	if v := TraceNumber("121042882"); v == "" {
		t.Error("empty trace number")
	}
}

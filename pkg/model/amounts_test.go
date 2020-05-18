// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"testing"
)

func TestSumAmounts(t *testing.T) {
	sum, err := SumAmounts(
		"USD 0.01",
		"USD 11.34",
		"USD 5.21",
	)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Int() != 1656 {
		t.Errorf("got %q", sum)
	}

	sum, err = SumAmounts()
	if err != nil {
		t.Fatal(err)
	}
	if sum.Int() != 0 {
		t.Errorf("got %q", sum)
	}
}

func TestSumAmountsErr(t *testing.T) {
	sum, err := SumAmounts("invalid")
	if err == nil {
		t.Error("expected error")
	}
	if sum != nil {
		t.Errorf("unexpected total=%q", sum)
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"fmt"
	"testing"
)

func between(amt string) error {
	if amt >= "USD 0.01" && amt <= "USD 0.25" {
		return nil
	}
	return fmt.Errorf("invalid amount %q", amt)
}

func TestAmountConditions(t *testing.T) {
	if err := between("USD 0.10"); err != nil {
		t.Error(err)
	}
	if err := between("USD 0.24"); err != nil {
		t.Error(err)
	}

	if err := between("USD 0.00"); err == nil {
		t.Error("expected error")
	}
	if err := between("USD 0.26"); err == nil {
		t.Error("expected error")
	}

	if err := between(""); err == nil {
		t.Error("expected error")
	}
	if err := between("invalid"); err == nil {
		t.Error("expected error")
	}
}

func TestAmounts(t *testing.T) {
	amt1, amt2 := getMicroDepositAmounts()
	if err := between(amt1); err != nil {
		t.Error(err)
	}
	if err := between(amt2); err != nil {
		t.Error(err)
	}
}

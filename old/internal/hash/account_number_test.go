// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package hash

import (
	"testing"
)

func TestAccountNumber(t *testing.T) {
	if num, err := AccountNumber("1234"); err != nil {
		t.Fatal(err)
	} else {
		if num != "03ac674216f3e15c761ee1a5e255f067953623c8b388b4459e13f978d7c846f4" {
			t.Errorf("got %s", num)
		}
	}
}

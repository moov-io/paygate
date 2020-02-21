// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"testing"
)

func TestConvertAddress(t *testing.T) {
	if xs := ConvertAddress(nil); len(xs) != 0 {
		t.Errorf("got addresses=%#v", xs)
	}

	addresses := ConvertAddress(&Address{
		Address1:   "address1",
		Address2:   "address2",
		City:       "city",
		State:      "state",
		PostalCode: "90210",
	})
	if len(addresses) != 1 {
		t.Errorf("got addresses=%#v", addresses)
	}
	if addresses[0].Address1 != "address1" {
		t.Errorf("addresses[0].Address1=%s", addresses[0].Address1)
	}
	if addresses[0].Country != "US" {
		t.Errorf("addresses[0].Country=%s", addresses[0].Country)
	}
}

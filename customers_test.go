// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCustomerStatus__json(t *testing.T) {
	cs := CustomerStatus("invalid")
	valid := map[string]CustomerStatus{
		"unverified":  CustomerUnverified,
		"verified":    CustomerVerified,
		"suspended":   CustomerSuspended,
		"deactivated": CustomerDeactivated,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &cs); err != nil {
			t.Error(err.Error())
		}
		if cs != v {
			t.Errorf("got cs=%#v, v=%#v", cs, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, nextID()))
	if err := json.Unmarshal(in, &cs); err == nil {
		t.Error("expected error")
	}
}

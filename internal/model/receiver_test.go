// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moov-io/base"
)

func TestReceiverStatus__json(t *testing.T) {
	cs := ReceiverStatus("invalid")
	valid := map[string]ReceiverStatus{
		"unverified":  ReceiverUnverified,
		"verIFIed":    ReceiverVerified,
		"SUSPENDED":   ReceiverSuspended,
		"deactivated": ReceiverDeactivated,
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
	in := []byte(fmt.Sprintf(`"%v"`, base.ID()))
	if err := json.Unmarshal(in, &cs); err == nil {
		t.Error("expected error")
	}
}

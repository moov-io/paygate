// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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

func TestReceiver__json(t *testing.T) {
	id := base.ID()
	now := time.Now()

	response := Receiver{
		ID:        ReceiverID(id),
		BirthDate: &now,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&response); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), fmt.Sprintf(`"id":"%s"`, id)) {
		t.Errorf("missing id: %s", buf.String())
	}
	if strings.Contains(buf.String(), `"birthDate":null`) {
		t.Errorf("missing birthDate: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"address":null`) {
		t.Errorf("missing address: %s", buf.String())
	}

	// marshal without BirthDate, but with Address
	response.BirthDate = nil
	response.Address = &Address{Address1: "foo"}
	if err := json.NewEncoder(&buf).Encode(&response); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), fmt.Sprintf(`"id":"%s"`, id)) {
		t.Errorf("missing id: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"birthDate":null`) {
		t.Errorf("expected no birthDate: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"address":{"address1":"foo"`) {
		t.Errorf("expected address: %s", buf.String())
	}
}

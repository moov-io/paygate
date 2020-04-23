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

func TestOriginator__json(t *testing.T) {
	id := base.ID()
	now := time.Now()

	response := Originator{
		ID:        OriginatorID(id),
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

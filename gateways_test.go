// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestGateways_getUserGateways(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/gateways", nil)
	r.Header.Set("x-user-id", "test")

	getUserGateways(memGatewayRepo{})(w, r)
	w.Flush()

	if w.Code != 200 {
		t.Errorf("got %d", w.Code)
	}

	var gateways []*Gateway
	if err := json.Unmarshal(w.Body.Bytes(), &gateways); err != nil {
		t.Error(err)
	}
	if len(gateways) != 1 {
		t.Errorf("got %d gateways=%v", len(gateways), gateways)
	}
	if gateways[0].ID == "" {
		t.Errorf("gateways[0]=%v", gateways[0])
	}
}

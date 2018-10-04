// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestOriginators_getUserOriginators(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/originators", nil)
	r.Header.Set("x-user-id", "test")

	getUserOriginators(memOriginatorRepo{})(w, r)
	w.Flush()

	if w.Code != 200 {
		t.Errorf("got %d", w.Code)
	}

	var originators []*Originator
	if err := json.Unmarshal(w.Body.Bytes(), &originators); err != nil {
		t.Error(err)
	}
	if len(originators) != 1 {
		t.Errorf("got %d originators=%v", len(originators), originators)
	}
	if originators[0].ID == "" {
		t.Errorf("originators[0]=%v", originators[0])
	}
}

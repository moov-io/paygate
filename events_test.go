// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestEvents__getUserEvents(t *testing.T) {
	// happy path
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/events", nil)
	r.Header.Set("x-user-id", "test")

	getUserEvents(memEventRepo{})(w, r)
	w.Flush()

	if w.Code != 200 {
		t.Errorf("got %d", w.Code)
	}

	var events []*Event
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Error(err)
	}
	if len(events) != 1 {
		t.Errorf("got %d events=%v", len(events), events)
	}
	if events[0].ID == "" {
		t.Errorf("events[0]=%v", events[0])
	}
}

func TestEvents__getEventHandler(t *testing.T) {

}

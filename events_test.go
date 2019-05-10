// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/moov-io/base"

	"github.com/go-kit/kit/log"
)

func TestEvents__getUserEvents(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	repo := &sqliteEventRepo{
		db:  db.db,
		log: log.NewNopLogger(),
	}

	// Write a sample event
	userId := base.ID()
	event := &Event{
		ID:      EventID(base.ID()),
		Topic:   "testing",
		Message: "This is a test",
		Type:    "TestEvent",
	}
	if err := repo.writeEvent(userId, event); err != nil {
		t.Fatal(err)
	}

	// happy path
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/events", nil)
	r.Header.Set("x-user-id", userId)

	getUserEvents(log.NewNopLogger(), repo)(w, r)
	w.Flush()

	if w.Code != 200 {
		t.Errorf("got %d", w.Code)
	}

	var events []*Event
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Error(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events=%v", len(events), events)
	}
	if events[0].ID == "" {
		t.Errorf("events[0]=%v", events[0])
	}
}

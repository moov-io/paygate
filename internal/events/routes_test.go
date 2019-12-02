// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestEvents__getUserEvents(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
		userID := id.User(base.ID())
		event := &Event{
			ID:      EventID(base.ID()),
			Topic:   "testing",
			Message: "This is a test",
			Type:    "TestEvent",
		}
		if err := repo.WriteEvent(userID, event); err != nil {
			t.Fatal(err)
		}

		router := mux.NewRouter()
		AddRoutes(log.NewNopLogger(), router, repo)

		req, _ := http.NewRequest("GET", "/events", nil)
		req.Header.Set("x-user-id", userID.String())

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		w.Flush()

		if w.Code != http.StatusOK {
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

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewRepo(log.NewNopLogger(), sqliteDB.DB))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewRepo(log.NewNopLogger(), mysqlDB.DB))
}

func TestEvents__getEvent(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
		userID := id.User(base.ID())
		event := &Event{
			ID:      EventID(base.ID()),
			Topic:   "testing",
			Message: "This is a test",
			Type:    "TestEvent",
		}
		if err := repo.WriteEvent(userID, event); err != nil {
			t.Fatal(err)
		}

		router := mux.NewRouter()
		AddRoutes(log.NewNopLogger(), router, repo)

		req, _ := http.NewRequest("GET", fmt.Sprintf("/events/%s", event.ID), nil)
		req.Header.Set("x-user-id", userID.String())

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		w.Flush()

		if w.Code != http.StatusOK {
			t.Errorf("got %d", w.Code)
		}

		var wrapper Event
		if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
			t.Fatal(err)
		}
		if wrapper.ID != event.ID {
			t.Errorf("wrapper.ID=%s", wrapper.ID)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewRepo(log.NewNopLogger(), sqliteDB.DB))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewRepo(log.NewNopLogger(), mysqlDB.DB))
}

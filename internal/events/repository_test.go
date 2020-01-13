// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package events

import (
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

func TestSQLRepository(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLRepository) {
		defer repo.Close()

		eventID, userID := EventID(base.ID()), id.User(base.ID())

		if event, err := repo.GetEvent(eventID, userID); event != nil || err != nil {
			t.Fatalf("expected nil event=%v: %v", event, err)
		}
		if events, err := repo.GetUserEvents(userID); len(events) != 0 || err != nil {
			t.Fatalf("expected nil events=%v: %v", events, err)
		}

		metadata := make(map[string]string)
		metadata["transferID"] = base.ID()
		metadata["salesforceID"] = base.ID()

		evt := &Event{
			ID:       eventID,
			Topic:    "test",
			Message:  "testing",
			Type:     TransferEvent,
			Metadata: metadata,
		}
		if err := repo.WriteEvent(userID, evt); err != nil {
			t.Fatal(err)
		}

		if event, err := repo.GetEvent(eventID, userID); event == nil || err != nil {
			t.Fatalf("expected nil event=%v: %v", event, err)
		} else {
			if event.ID != eventID {
				t.Errorf("unexpected event: %v", event)
			}
			if event.Metadata["transferID"] == "" {
				t.Errorf("transferID=%s", event.Metadata["transferID"])
			}
		}
		if events, err := repo.GetUserEvents(userID); len(events) != 1 || err != nil {
			t.Fatalf("expected nil events=%v: %v", events, err)
		} else {
			if events[0].ID != eventID {
				t.Errorf("unexpected event: %v", events[0])
			}
			if events[0].Metadata["transferID"] == "" {
				t.Errorf("transferID=%s", events[0].Metadata["transferID"])
			}
		}
	}

	// SQLite
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, NewRepo(log.NewNopLogger(), sqliteDB.DB))

	// MySQL
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewRepo(log.NewNopLogger(), mysqlDB.DB))
}

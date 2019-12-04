// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package events

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

type Repository interface {
	GetEvent(eventID EventID, userID id.User) (*Event, error)
	GetUserEvents(userID id.User) ([]*Event, error)

	GetUserEventsByMetadata(userID id.User, metadata map[string]string) ([]*Event, error)

	WriteEvent(userID id.User, event *Event) error
}

func NewRepo(logger log.Logger, db *sql.DB) *SQLRepository {
	return &SQLRepository{log: logger, db: db}
}

type SQLRepository struct {
	db  *sql.DB
	log log.Logger
}

func (r *SQLRepository) Close() error {
	return r.db.Close()
}

func (r *SQLRepository) WriteEvent(userID id.User, event *Event) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("write event: begin: error=%v rollback=%v", err, tx.Rollback())
	}

	query := `insert into events (event_id, user_id, topic, message, type, created_at) values (?, ?, ?, ?, ?, ?)`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("write event: prepare: error=%v rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()

	_, err = stmt.Exec(event.ID, userID, event.Topic, event.Message, event.Type, time.Now())
	if err != nil {
		return fmt.Errorf("write event: exec: error=%v rollback=%v", err, tx.Rollback())
	}

	query = "insert into event_metadata (event_id, user_id, `key`, value) values (?, ?, ?, ?);"
	stmt, err = tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("write event: metadata prepare: error=%v rollback=%v", err, tx.Rollback())
	}
	for k, v := range event.Metadata {
		if _, err := stmt.Exec(event.ID, userID, k, v); err != nil {
			return fmt.Errorf("write event metadata: error=%v rollback=%v", err, tx.Rollback())
		}
	}
	return tx.Commit()
}

func (r *SQLRepository) GetEvent(eventID EventID, userID id.User) (*Event, error) {
	query := `select event_id, topic, message, type from events
where event_id = ? and user_id = ?
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(eventID, userID)

	var event Event
	if err := row.Scan(&event.ID, &event.Topic, &event.Message, &event.Type); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // not found
		}
		return nil, err
	}
	if event.ID == "" {
		return nil, nil // event not found
	}
	event.Metadata = r.getEventMetadata(event.ID)
	return &event, nil
}

func (r *SQLRepository) GetUserEvents(userID id.User) ([]*Event, error) {
	query := `select event_id from events where user_id = ?`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var eventIDs []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return nil, fmt.Errorf("getUserEvents scan: %v", err)
		}
		if row != "" {
			eventIDs = append(eventIDs, row)
		}
	}
	var events []*Event
	for i := range eventIDs {
		event, err := r.GetEvent(EventID(eventIDs[i]), userID)
		if err == nil && event != nil {
			events = append(events, event)
		}
	}
	return events, rows.Err()
}

func (r *SQLRepository) GetUserEventsByMetadata(userID id.User, metadata map[string]string) ([]*Event, error) {
	query := `select distinct event_id from event_metadata where user_id = ?` + strings.Repeat(` and key = ? and value = ?`, len(metadata))
	var args = []interface{}{userID.String()}
	for k, v := range metadata {
		args = append(args, k, v)
	}
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("get events by metadata: prepare: %v", err)
	}

	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, fmt.Errorf("get events by metadata: query: %v", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		id := ""
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("get events by metadata: scan: %v", err)
		}
		if evt, err := r.GetEvent(EventID(id), userID); err != nil {
			return nil, fmt.Errorf("get events by metadata: get: %v", err)
		} else {
			events = append(events, evt)
		}
	}
	return events, nil
}

func (r *SQLRepository) getEventMetadata(eventID EventID) map[string]string {
	query := "select `key`, value from event_metadata where event_id = ?;"
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil
	}

	rows, err := stmt.Query(eventID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		key, value := "", ""
		if err := rows.Scan(&key, &value); err != nil {
			return nil
		}
		if key != "" {
			out[key] = value
		}
	}
	return out
}

// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	moovhttp "github.com/moov-io/base/http"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type EventID string

type Event struct {
	ID      EventID   `json:"id"`
	Topic   string    `json:"topic"`
	Message string    `json:"message"`
	Type    EventType `json:"type"`

	// TODO(adam): We might need to inspect/filter events by metadata
	// map[string]string "transferId" -> "..."
}

type EventType string

const (
	// TODO(adam): more EventType values?
	// ReceiverEvent   EventType = "Receiver"
	// DepositoryEvent EventType = "Depository"
	// OriginatorEvent EventType = "Originator"
	TransferEvent EventType = "Transfer"
)

func addEventRoutes(logger log.Logger, r *mux.Router, eventRepo eventRepository) {
	r.Methods("GET").Path("/events").HandlerFunc(getUserEvents(logger, eventRepo))
	r.Methods("GET").Path("/events/{eventId}").HandlerFunc(getEventHandler(logger, eventRepo))
}

func getUserEvents(logger log.Logger, eventRepo eventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		userId := moovhttp.GetUserId(r)
		events, err := eventRepo.getUserEvents(userId)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(events); err != nil {
			internalError(logger, w, err)
			return
		}
	}
}

func getEventHandler(logger log.Logger, eventRepo eventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		eventId, userId := getEventId(r), moovhttp.GetUserId(r)
		if eventId == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// grab event
		event, err := eventRepo.getEvent(eventId, userId)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(event); err != nil {
			internalError(logger, w, err)
			return
		}
	}
}

// getEventId extracts the EventID from the incoming request.
func getEventId(r *http.Request) EventID {
	v := mux.Vars(r)
	id, ok := v["eventId"]
	if !ok {
		return EventID("")
	}
	return EventID(id)
}

type eventRepository interface {
	getEvent(eventId EventID, userId string) (*Event, error)
	getUserEvents(userId string) ([]*Event, error)

	writeEvent(userId string, event *Event) error

	getUserTransferEvents(userId string, transferId TransferID) ([]*Event, error)
}

type sqliteEventRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *sqliteEventRepo) close() error {
	return r.db.Close()
}

func (r *sqliteEventRepo) writeEvent(userId string, event *Event) error {
	query := `insert into events (event_id, user_id, topic, message, type, created_at) values (?, ?, ?, ?, ?, ?)`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(event.ID, userId, event.Topic, event.Message, event.Type, time.Now())
	if err != nil {
		return err
	}
	return nil
}

func (r *sqliteEventRepo) getEvent(eventId EventID, userId string) (*Event, error) {
	query := `select event_id, topic, message, type from events
where event_id = ? and user_id = ?
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(eventId, userId)

	event := &Event{}
	err = row.Scan(&event.ID, &event.Topic, &event.Message, &event.Type)
	if err != nil {
		return nil, err
	}
	if event.ID == "" {
		return nil, nil // event not found
	}
	return event, nil
}

func (r *sqliteEventRepo) getUserEvents(userId string) ([]*Event, error) {
	query := `select event_id from events where user_id = ?`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var eventIds []string
	for rows.Next() {
		var row string
		rows.Scan(&row)
		if row != "" {
			eventIds = append(eventIds, row)
		}
	}
	var events []*Event
	for i := range eventIds {
		event, err := r.getEvent(EventID(eventIds[i]), userId)
		if err == nil && event != nil {
			events = append(events, event)
		}
	}
	return events, rows.Err()
}

func (r *sqliteEventRepo) getUserTransferEvents(userId string, id TransferID) ([]*Event, error) {
	// TODO(adam): need to store transferId alongside in some arbitrary json
	// Scan on Type == TransferEvent ?
	return nil, nil
}

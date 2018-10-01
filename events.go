// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

type EventID string

type Event struct {
	ID      EventID   `json:"id"`
	Topic   string    `json:"topic"`
	Message string    `json:"message"`
	Type    EventType `json:"type"`

	// optional
	transferId string
}

type EventType string

const (
	CustomerEvent   EventType = "Customer"
	DepositoryEvent           = "Depository"
	OriginatorEvent           = "Originator"
	TransferEvent             = "Transfer"
)

func addEventRoutes(r *mux.Router, eventRepo eventRepository) {
	r.Methods("GET").Path("/events").HandlerFunc(getUserEvents(eventRepo))
	r.Methods("GET").Path("/events/{eventId}").HandlerFunc(getEventHandler(eventRepo))
}

func getUserEvents(eventRepo eventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserEvents")
		if err != nil {
			return
		}

		userId := getUserId(r)
		events, err := eventRepo.GetUserEvents(userId)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(events); err != nil {
			internalError(w, err, "events")
			return
		}
	}
}

func getEventHandler(eventRepo eventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getEventHandler")
		if err != nil {
			return
		}

		eventId, userId := getEventId(r), getUserId(r)
		if eventId == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// grab event
		event, err := eventRepo.GetEvent(eventId, userId)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(event); err != nil {
			internalError(w, err, "events")
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
	GetEvent(eventId EventID, userId string) (*Event, error)
	GetUserEvents(userId string) ([]*Event, error)

	GetUserTransferEvents(userId string, transferId TransferID) ([]*Event, error)
}

type memEventRepo struct{}

func (memEventRepo) GetEvent(eventId EventID, userId string) (*Event, error) {
	return &Event{
		ID:      eventId,
		Topic:   "paygate test event",
		Message: "This is a test!",
		Type:    CustomerEvent,
	}, nil
}

func (m memEventRepo) GetUserEvents(userId string) ([]*Event, error) {
	event, err := m.GetEvent(EventID(nextID()), userId)
	if err != nil {
		return nil, err
	}
	return []*Event{event}, nil
}

func (m memEventRepo) GetUserTransferEvents(userId string, id TransferID) ([]*Event, error) {
	events, err := m.GetUserEvents(userId)
	if err != nil {
		return nil, err
	}

	events = append(events, &Event{
		ID:         EventID(nextID()),
		Topic:      "Transfer started",
		Type:       TransferEvent,
		transferId: string(id),
	})

	var kept []*Event
	for i := range events {
		if id.Equal(events[i].transferId) {
			kept = append(kept, events[i])
		}
	}
	return kept, nil
}

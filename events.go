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
		events := eventRepo.GetUserEvents(userId)

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
		if event := eventRepo.GetEvent(eventId, userId); event != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)

			if err := json.NewEncoder(w).Encode(event); err != nil {
				internalError(w, err, "events")
				return
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
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
	GetEvent(eventId EventID, userId string) *Event
	GetUserEvents(userId string) []*Event
}

type memEventRepo struct{}

func (memEventRepo) GetEvent(eventId EventID, userId string) *Event {
	return &Event{
		ID:      eventId,
		Topic:   "paygate test event",
		Message: "This is a test!",
		Type:    CustomerEvent,
	}
}

func (m memEventRepo) GetUserEvents(userId string) []*Event {
	return []*Event{
		m.GetEvent(EventID(nextID()), userId),
	}
}

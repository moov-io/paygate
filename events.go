// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
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

func addEventRoutes(r *mux.Router) {
	r.Methods("GET").Path("/events").HandlerFunc(getUserEvents)
	r.Methods("GET").Path("/events/{eventId}").HandlerFunc(getEventHandler)
}

func getUserEvents(w http.ResponseWriter, r *http.Request) {
	ww := wrapResponseWriter(w, routeHistogram, []string{"route", "getUserEvents"})
	if err := ww.ensureHeaders(r); err != nil {
		return
	}

	// userId := getUserId(r)
	// TODO(adam): find events for a user_id

	ww.WriteHeader(http.StatusOK)
}

func getEventHandler(w http.ResponseWriter, r *http.Request) {

}

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package events

import (
	"encoding/json"
	"net/http"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/route"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func AddRoutes(logger log.Logger, r *mux.Router, eventRepo Repository) {
	r.Methods("GET").Path("/events").HandlerFunc(getUserEvents(logger, eventRepo))
	r.Methods("GET").Path("/events/{eventID}").HandlerFunc(getEventHandler(logger, eventRepo))
}

func getUserEvents(logger log.Logger, eventRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		events, err := eventRepo.GetUserEvents(responder.XUserID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}
		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(events)
		})
	}
}

func getEventHandler(logger log.Logger, eventRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		eventID := getEventID(r)
		if eventID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// grab event
		event, err := eventRepo.GetEvent(eventID, responder.XUserID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(event)
		})
	}
}

// getEventID extracts the EventID from the incoming request.
func getEventID(r *http.Request) EventID {
	v := mux.Vars(r)
	id, ok := v["eventID"]
	if !ok {
		return EventID("")
	}
	return EventID(id)
}

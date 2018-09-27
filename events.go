// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
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

}

// GET /events
// GET /events/{id}

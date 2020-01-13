// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package events

type EventID string

type Event struct {
	ID      EventID   `json:"id"`
	Topic   string    `json:"topic"`
	Message string    `json:"message"`
	Type    EventType `json:"type"`

	Metadata map[string]string `json:"metadata"`
}

type EventType string

const (
	// TODO(adam): more EventType values?
	// ReceiverEvent   EventType = "Receiver"
	// DepositoryEvent EventType = "Depository"
	// OriginatorEvent EventType = "Originator"
	TransferEvent EventType = "Transfer"
)

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/pkg/id"
)

func (c *TransferRouter) getUserTransferEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		if responder == nil {
			return
		}

		transferID := getTransferID(r)
		transfer, err := c.transferRepo.getUserTransfer(transferID, responder.XUserID)
		if transfer == nil || err != nil {
			responder.Problem(err)
			return
		}

		metadata := make(map[string]string)
		metadata["transferID"] = fmt.Sprintf("%v", transfer.ID)

		events, err := c.eventRepo.GetUserEventsByMetadata(responder.XUserID, metadata)
		if err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(events)
		})
	}
}

func writeTransferEvent(userID id.User, req *transferRequest, eventRepo events.Repository) error {
	return eventRepo.WriteEvent(userID, &events.Event{
		ID:      events.EventID(base.ID()),
		Topic:   fmt.Sprintf("%s transfer to %s", req.Type, req.Description),
		Message: req.Description,
		Type:    events.TransferEvent,
	})
}

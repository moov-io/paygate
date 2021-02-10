// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/moov-io/base/log"

	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/transfers"
	"github.com/moov-io/paygate/x/route"
)

func getTransferID(r *http.Request) string {
	return route.ReadPathID("transferID", r)
}

func updateTransferStatus(cfg *config.Config, repo transfers.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(cfg, w, r)

		var request struct {
			Status client.TransferStatus `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			responder.Problem(err)
			return
		}

		transferID := getTransferID(r)
		existing, err := repo.GetTransfer(transferID)
		if err != nil {
			responder.Problem(fmt.Errorf("initial read: %v", err))
			return
		}
		if existing == nil {
			responder.Problem(errors.New("transfer not found"))
			return
		}
		if err := validStatusTransistion(existing.TransferID, existing.Status, request.Status); err != nil {
			responder.Problem(err)
			return
		}

		// Perform the DB update since it's an allowed transition
		if err := repo.UpdateTransferStatus(transferID, request.Status); err != nil {
			responder.Problem(err)
			return
		}
		cfg.Logger.With(log.Fields{
			"requestID":    log.String(responder.XRequestID),
			"organization": log.String(responder.OrganizationID),
			"transferID":   log.String(transferID),
			"status":       log.String(string(request.Status)),
		}).Log("Updated transfer status")

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
		})
	}
}

func validStatusTransistion(transferID string, current client.TransferStatus, proposed client.TransferStatus) error {
	// We only allow a couple of transitions for Transfer statuses as there are several
	switch current {
	case client.REVIEWABLE:
		// Reviewable transfers can only be moved to pending or canceled after a human has confirmed
		// the Transfer can be sent off.
		switch proposed {
		case client.CANCELED, client.PENDING:
			return nil // do nothing, allow status change
		default:
			return fmt.Errorf("unable to move transfer=%s into status=%s", transferID, proposed)
		}
	case client.PENDING:
		// Pending transfers can only be canceled as if they're already sent we can't undo that.
		switch proposed {
		case client.CANCELED, client.REVIEWABLE:
			return nil
		}
	}
	return fmt.Errorf("unable to move transfer=%s from status=%s to status=%s", transferID, current, proposed)
}

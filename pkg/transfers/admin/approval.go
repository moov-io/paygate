// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/id"
	"github.com/moov-io/paygate/pkg/transfers"
	"github.com/moov-io/paygate/x/route"

	"github.com/go-kit/kit/log"
)

func getTransferID(r *http.Request) id.Transfer {
	return id.Transfer(route.ReadPathID("transferID", r))
}

func updateTransferStatus(logger log.Logger, repo transfers.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

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
		logger.Log(
			"transfers", fmt.Sprintf("updated transfer=%s into status=%v", transferID, request.Status),
			"userID", responder.XUserID, "requestID", responder.XRequestID)

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
		})
	}
}

func validStatusTransistion(transferID string, incoming client.TransferStatus, proposed client.TransferStatus) error {
	// We only allow a couple of transitions for Transfer statuses as there are several
	switch incoming {
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
		//
		// TODO(adam): What if the transfer is already merged but not uploaded yet.. We'd need to remove it from the file.
		if proposed == client.CANCELED {
			return nil
		}
	}
	return fmt.Errorf("unable to move transfer=%s from status=%s to status=%s", transferID, incoming, proposed)
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"encoding/json"
	"fmt"
	"net/http"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/route"

	"github.com/go-kit/kit/log"
)

func updateTransferStatus(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w = route.Wrap(logger, w, r)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if r.Method != "PUT" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb: %s", r.Method))
			return
		}

		var request struct {
			Status model.TransferStatus `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		transferID := getTransferID(r)
		existing, err := repo.GetTransfer(transferID)
		if err != nil {
			moovhttp.Problem(w, fmt.Errorf("initial read: %v", err))
			return
		}

		// We only allow a couple of transitions for Transfer statuses as there are several
		switch existing.Status {
		case model.TransferReviewable:
			// Reviewable transfers can only be moved to pending or canceled after a human has confirmed
			// the Transfer can be sent off.
			switch request.Status {
			case model.TransferCanceled, model.TransferPending:
				// do nothing, allow status change

			default:
				moovhttp.Problem(w, fmt.Errorf("unable to move transfer=%s into status=%s", transferID, request.Status))
				return
			}

		case model.TransferPending:
			// Pending transfers can only be canceled as if they're already sent we can't undo that.
			//
			// TODO(adam): What if the transfer is already merged but not uploaded yet.. We'd need to remove it from the file.
			if request.Status != model.TransferCanceled {
				moovhttp.Problem(w, fmt.Errorf("unable to move transfer=%s into status=%s", transferID, request.Status))
				return
			}

		default:
			moovhttp.Problem(w, fmt.Errorf("unable to unable transfer=%s from status=%s", transferID, existing.Status))
			return
		}

		// Perform the DB update since it's an allowed transition
		if err := repo.UpdateTransferStatus(transferID, request.Status); err != nil {
			moovhttp.Problem(w, err)
			return
		}
		logger.Log(
			"transfers", fmt.Sprintf("updated transfer=%s into status=%v", transferID, request.Status),
			"userID", existing.UserID, "requestID", moovhttp.GetRequestID(r))

		xfer, err := repo.GetTransfer(transferID)
		if err != nil {
			moovhttp.Problem(w, fmt.Errorf("render read: %v", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(xfer)
	}
}

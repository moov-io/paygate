// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moov-io/ach"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/achx"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/pkg/id"
)

func (c *TransferRouter) validateUserTransfer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		if responder == nil {
			return
		}

		file, err := c.makeFileFromTransfer(responder.XUserID, getTransferID(r))
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}
		if err := file.Validate(); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (c *TransferRouter) getUserTransferFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		if responder == nil {
			return
		}

		file, err := c.makeFileFromTransfer(responder.XUserID, getTransferID(r))
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*ach.File{file})
		})
	}
}

func (c *TransferRouter) makeFileFromTransfer(userID id.User, transferID id.Transfer) (*ach.File, error) {
	transfer, err := c.transferRepo.getUserTransfer(transferID, userID)
	if err != nil {
		return nil, err
	}

	receiver, recDep, originator, origDep, err := c.getTransferObjects(userID,
		transfer.Originator, transfer.OriginatorDepository,
		transfer.Receiver, transfer.ReceiverDepository,
	)
	if err != nil {
		return nil, err
	}

	gateway, err := c.gatewayRepo.GetUserGateway(userID)
	if err != nil {
		return nil, err
	}
	if gateway == nil {
		return nil, errors.New("nil Gateway")
	}

	file, err := achx.ConstructFile(string(transferID), gateway, transfer, originator, origDep, receiver, recDep)
	if err != nil {
		return nil, err
	}

	if err := file.Create(); err != nil {
		return nil, err
	}

	return file, nil
}

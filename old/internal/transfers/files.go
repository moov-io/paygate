// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/remoteach"
	"github.com/moov-io/paygate/internal/route"
)

func (c *TransferRouter) validateUserTransfer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		if responder == nil {
			return
		}

		// Grab the id.Transfer and responder.XUserID
		transferId := getTransferID(r)
		fileID, err := c.transferRepo.GetFileIDForTransfer(transferId, responder.XUserID)
		if err != nil {
			responder.Log("transfers", fmt.Sprintf("error getting fileID for transfer=%s: %v", transferId, err))
			responder.Problem(err)
			return
		}
		if fileID == "" {
			responder.Problem(errors.New("transfer not found"))
			return
		}

		// Check our ACH file status/validity
		if err := remoteach.CheckFile(c.logger, c.achClientFactory(responder.XUserID), fileID, responder.XUserID); err != nil {
			responder.Problem(err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

func (c *TransferRouter) getUserTransferFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		if responder == nil {
			return
		}

		// Grab the id.Transfer and responder.XUserID
		transferId := getTransferID(r)
		fileID, err := c.transferRepo.GetFileIDForTransfer(transferId, responder.XUserID)
		if err != nil {
			responder.Log("transfers", fmt.Sprintf("error reading fileID for transfer=%s: %v", transferId, err))
			responder.Problem(err)
			return
		}
		if fileID == "" {
			responder.Problem(errors.New("transfer not found"))
			return
		}

		// Grab Transfer file(s)
		file, err := c.achClientFactory(responder.XUserID).GetFile(fileID)
		if err != nil {
			responder.Log("transfers", fmt.Sprintf("error getting ACH files for transfer=%s: %v", transferId, err))
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*ach.File{file})
		})
	}
}

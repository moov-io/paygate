// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"errors"
	"fmt"
	"net/http"

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

		// TODO(adam): create file and validate it
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

		// TODO(adam): create file and serve it

		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			// json.NewEncoder(w).Encode([]*ach.File{file})
		})
	}
}

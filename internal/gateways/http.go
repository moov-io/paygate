// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gateways

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/route"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type gatewayRequest struct {
	Origin          string `json:"origin"`
	OriginName      string `json:"originName"`
	Destination     string `json:"destination"`
	DestinationName string `json:"destinationName"`
}

func (r gatewayRequest) missingFields() error {
	if r.Origin == "" {
		return errors.New("missing gatewayRequest.Origin")
	}
	if r.OriginName == "" {
		return errors.New("missing gatewayRequest.OriginName")
	}
	if r.Destination == "" {
		return errors.New("missing gatewayRequest.Destination")
	}
	if r.DestinationName == "" {
		return errors.New("missing gatewayRequest.DestinationName")
	}
	return nil
}

func AddRoutes(logger log.Logger, r *mux.Router, gatewayRepo Repository) {
	r.Methods("GET").Path("/gateways").HandlerFunc(getUserGateway(logger, gatewayRepo))
	r.Methods("POST").Path("/gateways").HandlerFunc(createUserGateway(logger, gatewayRepo))
}

func getUserGateway(logger log.Logger, gatewayRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		gateway, err := gatewayRepo.getUserGateway(responder.XUserID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(gateway)
		})
	}
}

func createUserGateway(logger log.Logger, gatewayRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		var wrapper gatewayRequest
		if err := json.NewDecoder(internal.Read(r.Body)).Decode(&wrapper); err != nil {
			responder.Problem(err)
			return
		}
		if err := wrapper.missingFields(); err != nil {
			responder.Problem(fmt.Errorf("%v: %v", internal.ErrMissingRequiredJson, err))
			return
		}

		gateway, err := gatewayRepo.createUserGateway(responder.XUserID, wrapper)
		if err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(gateway)
		})
	}
}

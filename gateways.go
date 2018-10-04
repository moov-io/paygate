// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type GatewayID string

type Gateway struct {
	ID              GatewayID `json:"id"`
	Origin          string    `json:"origin"`
	OriginName      string    `json:"originName"`
	Destination     string    `json:"destination"`
	DestinationName string    `json:"destinationName"`
	Created         time.Time `json:"created"`
}

type gatewayRequest struct {
	Origin          string `json:"origin"`
	OriginName      string `json:"originName"`
	Destination     string `json:"destination"`
	DestinationName string `json:"destinationName"`
}

func (r gatewayRequest) missingFields() bool {
	return r.Origin == "" || r.OriginName == "" || r.Destination == "" || r.DestinationName == ""
}

func addGatewayRoutes(r *mux.Router, gatewayRepo gatewayRepository) {
	r.Methods("GET").Path("/gateways").HandlerFunc(getUserGateways(gatewayRepo))
	r.Methods("POST").Path("/gateways").HandlerFunc(createUserGateways(gatewayRepo))
}

func getUserGateways(gatewayRepo gatewayRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserGateways")
		if err != nil {
			return
		}

		userId := getUserId(r)
		gateways, err := gatewayRepo.getUserGateways(userId)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(gateways); err != nil {
			internalError(w, err, "getUserGateways")
			return
		}
	}
}

func createUserGateways(gatewayRepo gatewayRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserGateways")
		if err != nil {
			return
		}

		bs, err := read(r.Body)
		if err != nil {
			encodeError(w, err)
			return
		}
		var req gatewayRequest
		if err := json.Unmarshal(bs, &req); err != nil {
			encodeError(w, err)
			return
		}

		if req.missingFields() {
			encodeError(w, errMissingRequiredJson)
			return
		}

		userId := getUserId(r)
		gateway, err := gatewayRepo.createUsergateway(userId, req)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(gateway); err != nil {
			internalError(w, err, "createUserGateways")
			return
		}
	}
}

type gatewayRepository interface {
	getUserGateways(userId string) ([]*Gateway, error)

	createUsergateway(userId string, req gatewayRequest) (*Gateway, error)
}

type memGatewayRepo struct{}

func (r memGatewayRepo) createUsergateway(userId string, req gatewayRequest) (*Gateway, error) {
	return &Gateway{
		ID:              GatewayID(nextID()),
		Origin:          "origin",
		OriginName:      "origin name",
		Destination:     "destination",
		DestinationName: "destination name",
		Created:         time.Now(),
	}, nil
}

func (r memGatewayRepo) getUserGateways(userId string) ([]*Gateway, error) {
	g := &Gateway{
		ID:              GatewayID(nextID()),
		Origin:          "origin",
		OriginName:      "origin name",
		Destination:     "destination",
		DestinationName: "destination name",
		Created:         time.Now(),
	}
	return []*Gateway{g}, nil
}

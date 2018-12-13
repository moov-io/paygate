// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	moovhttp "github.com/moov-io/base/http"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type GatewayID string

type Gateway struct {
	// ID is a unique string representing this Gateway.
	ID GatewayID `json:"id"`

	// Origin is an ABA routing number
	Origin string `json:"origin"` // TODO(adam): validate

	// OriginName is the legal name associated with the origin routing number.
	OriginName string `json:"originName"`

	// Destination is an ABA routing number
	Destination string `json:"destination"` // TODO(adam): validate

	// DestinationName is the legal name associated with the destination routing number.
	DestinationName string `json:"destinationName"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created time.Time `json:"created"`
}

func (g *Gateway) validate() error {
	// TODO(adam): validate Origin and Destination
	if g.OriginName == "" {
		return errors.New("missing Gateway.OriginName")
	}
	if g.DestinationName == "" {
		return errors.New("missing Gateway.DestinationName")
	}
	return nil
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
	r.Methods("GET").Path("/gateways").HandlerFunc(getUserGateway(gatewayRepo))
	r.Methods("POST").Path("/gateways").HandlerFunc(createUserGateway(gatewayRepo))
}

func getUserGateway(gatewayRepo gatewayRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserGateway")
		if err != nil {
			return
		}

		userId := moovhttp.GetUserId(r)
		gateway, err := gatewayRepo.getUserGateway(userId)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(gateway); err != nil {
			internalError(w, err, "getUserGateway")
			return
		}
	}
}

func createUserGateway(gatewayRepo gatewayRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserGateway")
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

		userId := moovhttp.GetUserId(r)
		gateway, err := gatewayRepo.createUserGateway(userId, req)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(gateway); err != nil {
			internalError(w, err, "createUserGateway")
			return
		}
	}
}

type gatewayRepository interface {
	getUserGateway(userId string) (*Gateway, error)
	createUserGateway(userId string, req gatewayRequest) (*Gateway, error)
}

type sqliteGatewayRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *sqliteGatewayRepo) close() error {
	return r.db.Close()
}

func (r *sqliteGatewayRepo) createUserGateway(userId string, req gatewayRequest) (*Gateway, error) {
	gateway := &Gateway{
		Origin:          req.Origin,
		OriginName:      req.OriginName,
		Destination:     req.Destination,
		DestinationName: req.DestinationName,
		Created:         time.Now(),
	}
	if err := gateway.validate(); err != nil {
		return nil, err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}

	query := `select gateway_id from gateways where user_id = ? and deleted_at is null`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return nil, err
	}
	row := stmt.QueryRow(userId)

	var gatewayId string
	err = row.Scan(&gatewayId)
	if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
		return nil, err
	}
	if gatewayId == "" {
		gatewayId = nextID()
	}
	gateway.ID = GatewayID(gatewayId)

	// insert/update row
	query = `insert or replace into gateways (gateway_id, user_id, origin, origin_name, destination, destination_name, created_at) values (?, ?, ?, ?, ?, ?, ?)`
	stmt, err = tx.Prepare(query)
	if err != nil {
		return nil, err
	}

	_, err = stmt.Exec(gatewayId, userId, gateway.Origin, gateway.OriginName, gateway.Destination, gateway.DestinationName, gateway.Created)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return gateway, nil
}

func (r *sqliteGatewayRepo) getUserGateway(userId string) (*Gateway, error) {
	query := `select gateway_id, origin, origin_name, destination, destination_name, created_at
from gateways where user_id = ? and deleted_at is null limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	row := stmt.QueryRow(userId)

	gateway := &Gateway{}
	err = row.Scan(&gateway.ID, &gateway.Origin, &gateway.OriginName, &gateway.Destination, &gateway.DestinationName, &gateway.Created)
	if err != nil {
		return nil, err
	}
	if gateway.ID == "" {
		return nil, nil // not found
	}
	return gateway, nil
}

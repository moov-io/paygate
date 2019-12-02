// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type GatewayID string

type Gateway struct {
	// ID is a unique string representing this Gateway.
	ID GatewayID `json:"id"`

	// Origin is an ABA routing number
	Origin string `json:"origin"`

	// OriginName is the legal name associated with the origin routing number.
	OriginName string `json:"originName"`

	// Destination is an ABA routing number
	Destination string `json:"destination"`

	// DestinationName is the legal name associated with the destination routing number.
	DestinationName string `json:"destinationName"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`
}

func (g *Gateway) validate() error {
	if g == nil {
		return errors.New("nil Gateway")
	}

	// Origin
	if err := ach.CheckRoutingNumber(g.Origin); err != nil {
		return err
	}
	if g.OriginName == "" {
		return errors.New("missing Gateway.OriginName")
	}

	// Destination
	if err := ach.CheckRoutingNumber(g.Destination); err != nil {
		return err
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

func AddGatewayRoutes(logger log.Logger, r *mux.Router, gatewayRepo gatewayRepository) {
	r.Methods("GET").Path("/gateways").HandlerFunc(getUserGateway(logger, gatewayRepo))
	r.Methods("POST").Path("/gateways").HandlerFunc(createUserGateway(logger, gatewayRepo))
}

func getUserGateway(logger log.Logger, gatewayRepo gatewayRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

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

func createUserGateway(logger log.Logger, gatewayRepo gatewayRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		bs, err := read(r.Body)
		if err != nil {
			responder.Problem(err)
			return
		}
		var req gatewayRequest
		if err := json.Unmarshal(bs, &req); err != nil {
			responder.Problem(err)
			return
		}

		if err := req.missingFields(); err != nil {
			responder.Problem(fmt.Errorf("%v: %v", errMissingRequiredJson, err))
			return
		}

		userID := route.GetUserID(r)
		gateway, err := gatewayRepo.createUserGateway(userID, req)
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

type gatewayRepository interface {
	getUserGateway(userID id.User) (*Gateway, error)
	createUserGateway(userID id.User, req gatewayRequest) (*Gateway, error)
}

func NewGatewayRepo(logger log.Logger, db *sql.DB) *SQLGatewayRepo {
	return &SQLGatewayRepo{log: logger, db: db}
}

type SQLGatewayRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *SQLGatewayRepo) Close() error {
	return r.db.Close()
}

func (r *SQLGatewayRepo) createUserGateway(userID id.User, req gatewayRequest) (*Gateway, error) {
	gateway := &Gateway{
		Origin:          req.Origin,
		OriginName:      req.OriginName,
		Destination:     req.Destination,
		DestinationName: req.DestinationName,
		Created:         base.NewTime(time.Now()),
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
	defer stmt.Close()

	row := stmt.QueryRow(userID)

	var gatewayID string
	err = row.Scan(&gatewayID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("createUserGateway: scan error=%v rollback=%v", err, tx.Rollback())
	}
	if gatewayID == "" {
		gatewayID = base.ID()
	}
	gateway.ID = GatewayID(gatewayID)

	// insert/update row
	query = `insert into gateways (gateway_id, user_id, origin, origin_name, destination, destination_name, created_at) values (?, ?, ?, ?, ?, ?, ?)`
	stmt, err = tx.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("createUserGateway: prepare error=%v rollback=%v", err, tx.Rollback())
	}

	_, err = stmt.Exec(gatewayID, userID, gateway.Origin, gateway.OriginName, gateway.Destination, gateway.DestinationName, gateway.Created.Time)
	stmt.Close()
	if err != nil {
		// We need to update the row as it already exists.
		if database.UniqueViolation(err) {
			query = `update gateways set origin = ?, origin_name = ?, destination = ?, destination_name = ? where gateway_id = ? and user_id = ?`
			stmt, err = tx.Prepare(query)
			if err != nil {
				return nil, fmt.Errorf("createUserGateway: update: error=%v rollback=%v", err, tx.Rollback())
			}
			_, err = stmt.Exec(gateway.Origin, gateway.OriginName, gateway.Destination, gateway.DestinationName, gatewayID, userID)
			stmt.Close()
			if err != nil {
				return nil, fmt.Errorf("createUserGateway: update exec: error=%v rollback=%v", err, tx.Rollback())
			}
		} else {
			return nil, fmt.Errorf("createUserGateway: exec error=%v rollback=%v", err, tx.Rollback())
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return gateway, nil
}

func (r *SQLGatewayRepo) getUserGateway(userID id.User) (*Gateway, error) {
	query := `select gateway_id, origin, origin_name, destination, destination_name, created_at
from gateways where user_id = ? and deleted_at is null limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(userID)

	gateway := &Gateway{}
	var created time.Time
	err = row.Scan(&gateway.ID, &gateway.Origin, &gateway.OriginName, &gateway.Destination, &gateway.DestinationName, &created)
	if err != nil {
		return nil, err
	}
	gateway.Created = base.NewTime(created)
	if gateway.ID == "" {
		return nil, nil // not found
	}
	return gateway, nil
}

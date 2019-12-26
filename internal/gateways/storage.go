// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gateways

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

type Repository interface {
	getUserGateway(userID id.User) (*Gateway, error)
	createUserGateway(userID id.User, req gatewayRequest) (*Gateway, error)
}

func NewRepo(logger log.Logger, db *sql.DB) *SQLGatewayRepo {
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

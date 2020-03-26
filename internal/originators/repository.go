// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package originators

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type Repository interface {
	GetUserOriginators(userID id.User) ([]*model.Originator, error)
	GetUserOriginator(id model.OriginatorID, userID id.User) (*model.Originator, error)

	createUserOriginator(userID id.User, req originatorRequest) (*model.Originator, error)
	deleteUserOriginator(id model.OriginatorID, userID id.User) error
}

func NewOriginatorRepo(logger log.Logger, db *sql.DB) *SQLOriginatorRepo {
	return &SQLOriginatorRepo{log: logger, db: db}
}

type SQLOriginatorRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *SQLOriginatorRepo) Close() error {
	return r.db.Close()
}

func (r *SQLOriginatorRepo) GetUserOriginators(userID id.User) ([]*model.Originator, error) {
	query := `select originator_id from originators where user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var originatorIds []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return nil, fmt.Errorf("GetUserOriginators scan: %v", err)
		}
		if row != "" {
			originatorIds = append(originatorIds, row)
		}
	}

	var originators []*model.Originator
	for i := range originatorIds {
		orig, err := r.GetUserOriginator(model.OriginatorID(originatorIds[i]), userID)
		if err == nil && orig.ID != "" {
			originators = append(originators, orig)
		}
	}
	return originators, rows.Err()
}

func (r *SQLOriginatorRepo) GetUserOriginator(id model.OriginatorID, userID id.User) (*model.Originator, error) {
	query := `select originator_id, default_depository, identification, customer_id, metadata, created_at, last_updated_at
from originators
where originator_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userID)

	orig := &model.Originator{}
	var (
		created time.Time
		updated time.Time
	)
	err = row.Scan(&orig.ID, &orig.DefaultDepository, &orig.Identification, &orig.CustomerID, &orig.Metadata, &created, &updated)
	if err != nil {
		return nil, err
	}
	orig.Created = base.NewTime(created)
	orig.Updated = base.NewTime(updated)
	if orig.ID == "" {
		return nil, nil // not found
	}
	return orig, nil
}

func (r *SQLOriginatorRepo) createUserOriginator(userID id.User, req originatorRequest) (*model.Originator, error) {
	now := time.Now()
	orig := &model.Originator{
		ID:                model.OriginatorID(base.ID()),
		DefaultDepository: req.DefaultDepository,
		Identification:    req.Identification,
		CustomerID:        req.customerID,
		Metadata:          req.Metadata,
		Created:           base.NewTime(now),
		Updated:           base.NewTime(now),
	}
	if err := orig.Validate(); err != nil {
		return nil, err
	}

	query := `insert into originators (originator_id, user_id, default_depository, identification, customer_id, metadata, created_at, last_updated_at) values (?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	_, err = stmt.Exec(orig.ID, userID, orig.DefaultDepository, orig.Identification, orig.CustomerID, orig.Metadata, now, now)
	if err != nil {
		return nil, err
	}
	return orig, nil
}

func (r *SQLOriginatorRepo) deleteUserOriginator(id model.OriginatorID, userID id.User) error {
	query := `update originators set deleted_at = ? where originator_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(time.Now(), id, userID)
	return err
}

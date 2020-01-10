// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
)

// Cursor allows iterating through Originators and Receivers in ascending order to refresh
// their OFAC search on a set interval (weekly, monthly). This is done as part of NACHA rules.
//
// Currently this has knowledge of the originators and receivers DB structure, which isn't good
// going forward. We should split this out and not mix concerns between those domains and customers.
type Cursor struct {
	logger    log.Logger
	batchSize int
	db        *sql.DB // db needs to have access to 'originators' and 'receivers' table

	// originatorNewerThan and receiverNewerThan represents the minimum (oldest) created_at
	// value to return in each Next() call. The value starts at time.Zero and progresses towards
	// the current time as Customers are processed.
	originatorNewerThan time.Time
	receiverNewerThan   time.Time
}

func NewCursor(logger log.Logger, db *sql.DB, batchSize int) *Cursor {
	return &Cursor{
		logger:    logger,
		db:        db,
		batchSize: batchSize,
	}
}

// Cust is the metadata from an Originator or Receiver for a Moov Customer object
type Cust struct {
	ID        string
	CreatedAt time.Time

	OriginatorID         string
	OriginatorDepository string

	ReceiverID string
}

func (cur *Cursor) Close() error {
	if cur == nil || cur.db == nil {
		return nil
	}
	if err := cur.db.Close(); err != nil {
		return err
	}
	return nil
}

func (cur *Cursor) Next() ([]Cust, error) {
	origCustomers, err := cur.grabOriginatorBatch()
	if err != nil {
		return nil, fmt.Errorf("originators: %v", err)
	}
	recCustomers, err := cur.grabReceiverBatch()
	if err != nil {
		return nil, fmt.Errorf("receivers: %v", err)
	}
	return append(origCustomers, recCustomers...), nil
}

func (cur *Cursor) grabOriginatorBatch() ([]Cust, error) {
	query := `select originator_id, customer_id, created_at from originators where created_at > ? order by created_at asc`
	stmt, err := cur.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(cur.originatorNewerThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	max := cur.originatorNewerThan
	var out []Cust
	for rows.Next() {
		var cust Cust
		if err := rows.Scan(&cust.OriginatorID, &cust.ID, &cust.CreatedAt); err != nil {
			return nil, err
		}
		if cust.CreatedAt.After(max) {
			max = cust.CreatedAt // advance to latest timestamp
		}
		out = append(out, cust)
	}
	cur.originatorNewerThan = max
	return out, rows.Err()
}

func (cur *Cursor) grabReceiverBatch() ([]Cust, error) {
	query := `select receiver_id, customer_id, created_at from receivers where created_at > ? order by created_at asc`
	stmt, err := cur.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(cur.receiverNewerThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	max := cur.receiverNewerThan
	var out []Cust
	for rows.Next() {
		var cust Cust
		if err := rows.Scan(&cust.ReceiverID, &cust.ID, &cust.CreatedAt); err != nil {
			return nil, err
		}
		if cust.CreatedAt.After(max) {
			max = cust.CreatedAt // advance to latest timestamp
		}
		out = append(out, cust)
	}
	cur.receiverNewerThan = max
	return out, rows.Err()
}

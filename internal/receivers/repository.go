// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package receivers

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type Repository interface {
	getUserReceivers(userID id.User) ([]*model.Receiver, error)
	GetUserReceiver(id model.ReceiverID, userID id.User) (*model.Receiver, error)

	UpdateReceiverStatus(id model.ReceiverID, status model.ReceiverStatus) error

	UpsertUserReceiver(userID id.User, receiver *model.Receiver) error
	deleteUserReceiver(id model.ReceiverID, userID id.User) error
}

func NewReceiverRepo(logger log.Logger, db *sql.DB) *SQLReceiverRepo {
	return &SQLReceiverRepo{log: logger, db: db}
}

type SQLReceiverRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *SQLReceiverRepo) Close() error {
	return r.db.Close()
}

func (r *SQLReceiverRepo) getUserReceivers(userID id.User) ([]*model.Receiver, error) {
	query := `select receiver_id from receivers where user_id = ? and deleted_at is null`
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

	var receiverIDs []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return nil, fmt.Errorf("getUserReceivers scan: %v", err)
		}
		if row != "" {
			receiverIDs = append(receiverIDs, row)
		}
	}

	var receivers []*model.Receiver
	for i := range receiverIDs {
		receiver, err := r.GetUserReceiver(model.ReceiverID(receiverIDs[i]), userID)
		if err == nil && receiver != nil && receiver.Email != "" {
			receivers = append(receivers, receiver)
		}
	}
	return receivers, rows.Err()
}

func (r *SQLReceiverRepo) GetUserReceiver(id model.ReceiverID, userID id.User) (*model.Receiver, error) {
	query := `select receiver_id, email, default_depository, customer_id, status, metadata, created_at, last_updated_at
from receivers
where receiver_id = ?
and user_id = ?
and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userID)

	var receiver model.Receiver
	err = row.Scan(&receiver.ID, &receiver.Email, &receiver.DefaultDepository, &receiver.CustomerID, &receiver.Status, &receiver.Metadata, &receiver.Created.Time, &receiver.Updated.Time)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if receiver.ID == "" || receiver.Email == "" {
		return nil, nil // no records found
	}
	return &receiver, nil
}

func (r *SQLReceiverRepo) UpdateReceiverStatus(id model.ReceiverID, status model.ReceiverStatus) error {
	query := `update receivers set status = ?, last_updated_at = ? where receiver_id = ? and deleted_at is null;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(status, time.Now(), id); err != nil {
		return fmt.Errorf("error updating receiver=%s: %v", id, err)
	}
	return nil
}

func (r *SQLReceiverRepo) UpsertUserReceiver(userID id.User, receiver *model.Receiver) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	receiver.Updated = base.NewTime(time.Now().Truncate(1 * time.Second))

	query := `insert into receivers (receiver_id, user_id, email, default_depository, customer_id, status, metadata, created_at, last_updated_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("UpsertUserReceiver: prepare err=%v: rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()

	res, err := stmt.Exec(receiver.ID, userID, receiver.Email, receiver.DefaultDepository, receiver.CustomerID, receiver.Status, receiver.Metadata, receiver.Created.Time, receiver.Updated.Time)
	stmt.Close()
	if err != nil && !database.UniqueViolation(err) {
		return fmt.Errorf("problem upserting receiver=%q, userID=%q error=%v rollback=%v", receiver.ID, userID, err, tx.Rollback())
	}

	// Check and skip ahead if the insert failed (to database.UniqueViolation)
	if res != nil {
		if n, _ := res.RowsAffected(); n != 0 {
			return tx.Commit() // Receiver was inserted, so cleanup and exit
		}
	}
	query = `update receivers
set email = ?, default_depository = ?, customer_id = ?, status = ?, metadata = ?, last_updated_at = ?
where receiver_id = ? and user_id = ? and deleted_at is null`
	stmt, err = tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(receiver.Email, receiver.DefaultDepository, receiver.CustomerID, receiver.Status, receiver.Metadata, receiver.Updated.Time, receiver.ID, userID)
	stmt.Close()
	if err != nil {
		return fmt.Errorf("UpsertUserReceiver: exec error=%v rollback=%v", err, tx.Rollback())
	}
	return tx.Commit()
}

func (r *SQLReceiverRepo) deleteUserReceiver(id model.ReceiverID, userID id.User) error {
	// TODO(adam): Should this just change the status to Deactivated?
	query := `update receivers set deleted_at = ? where receiver_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(time.Now(), id, userID); err != nil {
		return fmt.Errorf("error deleting receiver_id=%q, user_id=%q: %v", id, userID, err)
	}
	return nil
}

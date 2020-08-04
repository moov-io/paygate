// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/model"
)

type Repository interface {
	getUserTransfers(userID string, params transferFilterParams) ([]*client.Transfer, error)
	GetTransfer(id string) (*client.Transfer, error)
	UpdateTransferStatus(transferID string, status client.TransferStatus) error
	WriteUserTransfer(userID string, transfer *client.Transfer) error
	deleteUserTransfer(userID string, transferID string) error

	SaveReturnCode(transferID string, returnCode string) error
	saveTraceNumbers(transferID string, traceNumbers []string) error
	LookupTransferFromReturn(amount *model.Amount, traceNumber string, effectiveEntryDate time.Time) (*client.Transfer, error)
}

func NewRepo(db *sql.DB) *sqlRepo {
	return &sqlRepo{db: db}
}

type sqlRepo struct {
	db *sql.DB
}

func (r *sqlRepo) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *sqlRepo) getUserTransfers(userID string, params transferFilterParams) ([]*client.Transfer, error) {
	var statusQuery string
	if string(params.Status) != "" {
		statusQuery = "and status = ?"
	}
	query := fmt.Sprintf(`select transfer_id from transfers
where user_id = ? and created_at >= ? and created_at <= ? and deleted_at is null %s
order by created_at desc limit ? offset ?;`, statusQuery)
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	args := []interface{}{userID, params.StartDate, params.EndDate, params.Limit, params.Offset}
	if statusQuery != "" {
		args = append(args, params.Status)
	}
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transferIDs []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return nil, fmt.Errorf("getUserTransfers scan: %v", err)
		}
		if row != "" {
			transferIDs = append(transferIDs, row)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("getUserTransfers: rows.Err=%v", err)
	}

	var transfers []*client.Transfer
	for i := range transferIDs {
		t, err := r.getUserTransfer(transferIDs[i], userID)
		if err == nil && t.TransferID != "" {
			transfers = append(transfers, t)
		}
	}
	return transfers, rows.Err()
}

func (r *sqlRepo) getUserTransfer(transferID string, userID string) (*client.Transfer, error) {
	query := `select transfer_id, amount, source_customer_id, source_account_id, destination_customer_id, destination_account_id, description, status, same_day, return_code, created_at
from transfers
where transfer_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(transferID, userID)

	transfer := &client.Transfer{}
	var (
		// amt        string
		returnCode *string
	)
	err = row.Scan(
		&transfer.TransferID,
		&transfer.Amount, // &amt,
		&transfer.Source.CustomerID,
		&transfer.Source.AccountID,
		&transfer.Destination.CustomerID,
		&transfer.Destination.AccountID,
		&transfer.Description,
		&transfer.Status,
		&transfer.SameDay,
		&returnCode,
		&transfer.Created,
	)
	if transfer.TransferID == "" || err != nil {
		return nil, err
	}
	if returnCode != nil {
		if rc := ach.LookupReturnCode(*returnCode); rc != nil {
			transfer.ReturnCode = client.ReturnCode{
				Code:        rc.Code,
				Reason:      rc.Reason,
				Description: rc.Description,
			}
		}
	}
	return transfer, nil
}

func (r *sqlRepo) GetTransfer(transferID string) (*client.Transfer, error) {
	query := `select user_id from transfers where transfer_id = ? and deleted_at is null limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	userID := ""
	if err := stmt.QueryRow(transferID).Scan(&userID); err != nil {
		return nil, err
	}
	return r.getUserTransfer(transferID, userID)
}

func (r *sqlRepo) UpdateTransferStatus(transferID string, status client.TransferStatus) error {
	query := `update transfers set status = ? where transfer_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(status, transferID)
	return err
}

func (r *sqlRepo) WriteUserTransfer(userID string, transfer *client.Transfer) error {
	query := `insert into transfers (transfer_id, user_id, amount, source_customer_id, source_account_id, destination_customer_id, destination_account_id, description, status, same_day, created_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		transfer.TransferID,
		userID,
		transfer.Amount,
		transfer.Source.CustomerID,
		transfer.Source.AccountID,
		transfer.Destination.CustomerID,
		transfer.Destination.AccountID,
		transfer.Description,
		transfer.Status,
		transfer.SameDay,
		time.Now(),
	)
	return err
}

func (r *sqlRepo) deleteUserTransfer(userID string, transferID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	query := `select status from transfers where transfer_id = ? and user_id = ? and deleted_at is null limit 1;`
	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	var status string
	if err := stmt.QueryRow(transferID, userID).Scan(&status); err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if !strings.EqualFold(status, string(client.PENDING)) {
		tx.Rollback()
		return fmt.Errorf("transferID=%s is not in PENDING status", transferID)
	}

	query = `update transfers set deleted_at = ?
where transfer_id = ? and user_id = ? and status = ? and deleted_at is null`
	stmt, err = tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(time.Now(), transferID, userID, client.PENDING)
	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	return tx.Commit()
}

func (r *sqlRepo) SaveReturnCode(transferID string, returnCode string) error {
	query := `update transfers set return_code = ? where transfer_id = ? and return_code is null and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(returnCode, transferID)
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}

func (r *sqlRepo) saveTraceNumbers(transferID string, traceNumbers []string) error {
	query := `insert into transfer_trace_numbers(transfer_id, trace_number) values (?, ?);`
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return err
	}
	for i := range traceNumbers {
		if _, err := stmt.Exec(transferID, traceNumbers[i]); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *sqlRepo) LookupTransferFromReturn(amount *model.Amount, traceNumber string, effectiveEntryDate time.Time) (*client.Transfer, error) {
	// To match returned files we take a few values which are assumed to uniquely identify a Transfer.
	// traceNumber, per NACHA guidelines, should be globally unique (routing number + random value),
	// but we are going to filter to only select Transfers created within a few days of the EffectiveEntryDate
	// to avoid updating really old (or future, I suppose) objects.
	query := `select xf.transfer_id, xf.user_id from transfers as xf
inner join transfer_trace_numbers trace on xf.transfer_id = trace.transfer_id
where xf.amount = ? and trace.trace_number = ? and xf.status = ? and (xf.created_at > ? and xf.created_at < ?) and xf.deleted_at is null limit 1`

	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	transferId, userID := "", ""
	min, max := startOfDayAndTomorrow(effectiveEntryDate)
	// Only include Transfer objects within 5 calendar days of the EffectiveEntryDate
	min = min.Add(-5 * 24 * time.Hour)
	max = max.Add(5 * 24 * time.Hour)

	row := stmt.QueryRow(amount.String(), traceNumber, client.PROCESSED, min, max)
	if err := row.Scan(&transferId, &userID); err != nil {
		return nil, err
	}

	return r.getUserTransfer(transferId, userID)
}

// startOfDayAndTomorrow returns two time.Time values from a given time.Time value.
// The first is at the start of the same day as provided and the second is exactly 24 hours
// after the first.
func startOfDayAndTomorrow(in time.Time) (time.Time, time.Time) {
	start := in.Truncate(24 * time.Hour)
	return start, start.Add(24 * time.Hour)
}

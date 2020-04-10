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
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

type Repository interface {
	getUserTransfers(userID id.User, params transferFilterParams) ([]*model.Transfer, error)
	GetTransfer(id id.Transfer) (*model.Transfer, error)
	getUserTransfer(id id.Transfer, userID id.User) (*model.Transfer, error)
	UpdateTransferStatus(id id.Transfer, status model.TransferStatus) error

	GetFileIDForTransfer(id id.Transfer, userID id.User) (string, error)
	GetTraceNumber(id id.Transfer) (string, error)

	LookupTransferFromReturn(sec string, amount *model.Amount, traceNumber string, effectiveEntryDate time.Time) (*model.Transfer, error)
	SetReturnCode(id id.Transfer, returnCode string) error

	// GetCursor returns a database cursor for Transfer objects that need to be
	// posted today.
	//
	// We currently default EffectiveEntryDate to tomorrow for any transfer and thus a
	// transfer created today needs to be posted.
	GetCursor(batchSize int, depRepo depository.Repository) *Cursor
	MarkTransferAsMerged(id id.Transfer, filename string, traceNumber string) error

	// MarkTransfersAsProcessed updates Transfers to Processed to signify they have been
	// uploaded to the ODFI. This needs to be done in one blocking operation to the caller.
	MarkTransfersAsProcessed(filename string, traceNumbers []string) (int64, error)

	createUserTransfers(userID id.User, requests []*transferRequest) ([]*model.Transfer, error)
	deleteUserTransfer(id id.Transfer, userID id.User) error
}

func NewTransferRepo(logger log.Logger, db *sql.DB) *SQLRepo {
	return &SQLRepo{log: logger, db: db}
}

type SQLRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *SQLRepo) Close() error {
	return r.db.Close()
}

func (r *SQLRepo) getUserTransfers(userID id.User, params transferFilterParams) ([]*model.Transfer, error) {
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

	var transfers []*model.Transfer
	for i := range transferIDs {
		t, err := r.getUserTransfer(id.Transfer(transferIDs[i]), userID)
		if err == nil && t.ID != "" {
			transfers = append(transfers, t)
		}
	}
	return transfers, rows.Err()
}

func (r *SQLRepo) GetTransfer(xferID id.Transfer) (*model.Transfer, error) {
	query := `select transfer_id, user_id from transfers where transfer_id = ? and deleted_at is null limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	transferID, userID := "", ""
	if err := stmt.QueryRow(xferID).Scan(&transferID, &userID); err != nil {
		return nil, err
	}
	return r.getUserTransfer(id.Transfer(transferID), id.User(userID))
}

func (r *SQLRepo) getUserTransfer(id id.Transfer, userID id.User) (*model.Transfer, error) {
	query := `select transfer_id, user_id, type, amount, originator_id, originator_depository, receiver, receiver_depository, description, standard_entry_class_code, status, same_day, return_code, created_at
from transfers
where transfer_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userID)

	transfer := &model.Transfer{}
	var (
		amt        string
		returnCode *string
		created    time.Time
	)
	err = row.Scan(&transfer.ID, &transfer.UserID, &transfer.Type, &amt, &transfer.Originator, &transfer.OriginatorDepository, &transfer.Receiver, &transfer.ReceiverDepository, &transfer.Description, &transfer.StandardEntryClassCode, &transfer.Status, &transfer.SameDay, &returnCode, &created)
	if err != nil {
		return nil, err
	}
	if returnCode != nil {
		transfer.ReturnCode = ach.LookupReturnCode(*returnCode)
	}
	transfer.Created = base.NewTime(created)
	// parse Amount struct
	if err := transfer.Amount.FromString(amt); err != nil {
		return nil, err
	}
	if transfer.ID == "" {
		return nil, nil // not found
	}
	return transfer, nil
}

func (r *SQLRepo) UpdateTransferStatus(id id.Transfer, status model.TransferStatus) error {
	query := `update transfers set status = ? where transfer_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(status, id)
	return err
}

func (r *SQLRepo) GetFileIDForTransfer(id id.Transfer, userID id.User) (string, error) {
	query := `select file_id from transfers where transfer_id = ? and user_id = ? limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userID)

	var fileID string
	if err := row.Scan(&fileID); err != nil {
		return "", err
	}
	return fileID, nil
}

func (r *SQLRepo) GetTraceNumber(id id.Transfer) (string, error) {
	query := `select trace_number from transfers where transfer_id = ? limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id)

	var traceNumber *string
	if err := row.Scan(&traceNumber); err != nil {
		return "", err
	}
	if traceNumber == nil {
		return "", nil
	}
	return *traceNumber, nil
}

func (r *SQLRepo) LookupTransferFromReturn(sec string, amount *model.Amount, traceNumber string, effectiveEntryDate time.Time) (*model.Transfer, error) {
	// To match returned files we take a few values which are assumed to uniquely identify a Transfer.
	// traceNumber, per NACHA guidelines, should be globally unique (routing number + random value),
	// but we are going to filter to only select Transfers created within a few days of the EffectiveEntryDate
	// to avoid updating really old (or future, I suppose) objects.
	query := `select transfer_id, user_id, transaction_id from transfers
where standard_entry_class_code = ? and amount = ? and trace_number = ? and status = ? and (created_at > ? and created_at < ?) and deleted_at is null limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	transferId, userID, transactionID := "", "", "" // holders for 'select ..'
	min, max := internal.StartOfDayAndTomorrow(effectiveEntryDate)
	// Only include Transfer objects within 5 calendar days of the EffectiveEntryDate
	min = min.Add(-5 * 24 * time.Hour)
	max = max.Add(5 * 24 * time.Hour)

	row := stmt.QueryRow(sec, amount.String(), traceNumber, model.TransferProcessed, min, max)
	if err := row.Scan(&transferId, &userID, &transactionID); err != nil {
		return nil, err
	}

	xfer, err := r.getUserTransfer(id.Transfer(transferId), id.User(userID))
	xfer.TransactionID = transactionID
	xfer.UserID = userID
	return xfer, err
}

func (r *SQLRepo) SetReturnCode(id id.Transfer, returnCode string) error {
	query := `update transfers set return_code = ? where transfer_id = ? and return_code is null and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(returnCode, id)
	return err
}

func (r *SQLRepo) createUserTransfers(userID id.User, requests []*transferRequest) ([]*model.Transfer, error) {
	query := `insert into transfers (transfer_id, user_id, type, amount, originator_id, originator_depository, receiver, receiver_depository, description, standard_entry_class_code, status, same_day, file_id, transaction_id, remote_address, created_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var transfers []*model.Transfer

	now := time.Now()
	var status model.TransferStatus = model.TransferPending
	for i := range requests {
		req, transferId := requests[i], base.ID()
		xfer := &model.Transfer{
			ID:                     id.Transfer(transferId),
			Type:                   req.Type,
			Amount:                 req.Amount,
			Originator:             req.Originator,
			OriginatorDepository:   req.OriginatorDepository,
			Receiver:               req.Receiver,
			ReceiverDepository:     req.ReceiverDepository,
			Description:            req.Description,
			StandardEntryClassCode: req.StandardEntryClassCode,
			Status:                 status,
			SameDay:                req.SameDay,
			Created:                base.NewTime(now),
		}
		if err := xfer.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed for transfer Originator=%s, Receiver=%s, Description=%s %v", xfer.Originator, xfer.Receiver, xfer.Description, err)
		}

		// write transfer
		_, err := stmt.Exec(transferId, userID, req.Type, req.Amount.String(), req.Originator, req.OriginatorDepository, req.Receiver, req.ReceiverDepository, req.Description, req.StandardEntryClassCode, status, req.SameDay, req.fileID, req.transactionID, req.remoteAddr, now)
		if err != nil {
			return nil, err
		}
		transfers = append(transfers, xfer)
	}
	return transfers, nil
}

func (r *SQLRepo) deleteUserTransfer(id id.Transfer, userID id.User) error {
	query := `update transfers set deleted_at = ? where transfer_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(time.Now(), id, userID)
	return err
}

func (r *SQLRepo) MarkTransfersAsProcessed(filename string, traceNumbers []string) (int64, error) {
	query := fmt.Sprintf(`update transfers set status = ?
where status = ? and merged_filename = ? and trace_number in (%s?) and deleted_at is null`, strings.Repeat("?, ", len(traceNumbers)-1))

	stmt, err := r.db.Prepare(query)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	args := []interface{}{model.TransferProcessed, model.TransferPending, filename}
	for i := range traceNumbers {
		args = append(args, traceNumbers[i])
	}

	res, err := stmt.Exec(args...)
	if res != nil {
		n, _ := res.RowsAffected()
		return n, err
	}
	return 0, err
}

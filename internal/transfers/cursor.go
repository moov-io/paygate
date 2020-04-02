// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"fmt"
	"time"

	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

// Cursor allows for iterating through Transfers in ascending order (by CreatedAt)
// to merge into files uploaded to an ODFI.
type Cursor struct {
	BatchSize int

	DepRepo      depository.Repository
	TransferRepo *SQLRepo

	// newerThan represents the minimum (oldest) created_at value to return in the batch.
	// The value starts at today's first instant and progresses towards time.Now() with each
	// batch by being set to the batch's newest time.
	newerThan time.Time
}

// GroupableTransfer holds metadata of a Transfer used in grouping for generating and merging ACH files
// to be uploaded into the Fed.
type GroupableTransfer struct {
	*model.Transfer

	// Destination is the ABA routing number of the Destination FI (DFI)
	// This comes from the Transfer's ReceiverDepository.RoutingNumber
	Destination string

	UserID id.User
}

// Next returns a slice of Transfer objects from the current day. Next should be called to process
// all objects for a given day in batches.
func (cur *Cursor) Next() ([]*GroupableTransfer, error) {
	query := `select transfer_id, user_id, created_at from transfers
where status = ? and (merged_filename is null or merged_filename = '') and created_at > ? and deleted_at is null
order by created_at asc limit ?`
	stmt, err := cur.TransferRepo.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("Cursor.Next: prepare: %v", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(model.TransferPending, cur.newerThan, cur.BatchSize) // only Pending transfers
	if err != nil {
		return nil, fmt.Errorf("Cursor.Next: query: %v", err)
	}
	defer rows.Close()

	type xfer struct {
		transferId, userID string
		createdAt          time.Time
	}
	var xfers []xfer
	for rows.Next() {
		var xf xfer
		if err := rows.Scan(&xf.transferId, &xf.userID, &xf.createdAt); err != nil {
			return nil, fmt.Errorf("Cursor.Next: scan: %v", err)
		}
		if xf.transferId != "" {
			xfers = append(xfers, xf)
		}
	}

	max := cur.newerThan

	var transfers []*GroupableTransfer
	for i := range xfers {
		t, err := cur.TransferRepo.getUserTransfer(id.Transfer(xfers[i].transferId), id.User(xfers[i].userID))
		if err != nil {
			continue
		}
		destDep, err := cur.DepRepo.GetUserDepository(t.ReceiverDepository, id.User(xfers[i].userID))
		if err != nil || destDep == nil {
			continue
		}
		transfers = append(transfers, &GroupableTransfer{
			Transfer:    t,
			Destination: destDep.RoutingNumber,
			UserID:      id.User(xfers[i].userID),
		})
		if xfers[i].createdAt.After(max) {
			max = xfers[i].createdAt // advance max to newest time
		}
	}
	cur.newerThan = max
	return transfers, rows.Err()
}

// GetCursor returns a Cursor for iterating through Transfers in ascending order (by CreatedAt)
// beginning at the start of the current day.
func (r *SQLRepo) GetCursor(batchSize int, depRepo depository.Repository) *Cursor {
	now := time.Now()
	return &Cursor{
		BatchSize:    batchSize,
		TransferRepo: r,
		newerThan:    time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		DepRepo:      depRepo,
	}
}

// MarkTransferAsMerged will set the merged_filename on Pending transfers so they aren't merged into multiple files
// and the file uploaded to the FED can be tracked.
func (r *SQLRepo) MarkTransferAsMerged(id id.Transfer, filename string, traceNumber string) error {
	query := `update transfers set merged_filename = ?, trace_number = ?
where status = ? and transfer_id = ? and (merged_filename is null or merged_filename = '') and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("MarkTransferAsMerged: transfer=%s filename=%s: %v", id, filename, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(filename, traceNumber, model.TransferPending, id)
	return err
}

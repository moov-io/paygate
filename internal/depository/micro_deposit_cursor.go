// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"fmt"
	"time"

	"github.com/moov-io/paygate/internal/model"
)

// MicroDepositCursor allows for iterating through micro-deposits in ascending order (by CreatedAt)
// to merge into files uploaded to an ODFI.
type MicroDepositCursor struct {
	BatchSize int

	DepRepo *SQLRepo

	// newerThan represents the minimum (oldest) created_at value to return in the batch.
	// The value starts at today's first instant and progresses towards time.Now() with each
	// batch by being set to the batch's newest time.
	newerThan time.Time
}

type UploadableMicroDeposit struct {
	DepositoryID string
	UserID       string
	Amount       *model.Amount
	FileID       string
	CreatedAt    time.Time
}

// Next returns a slice of micro-deposit objects from the current day. Next should be called to process
// all objects for a given day in batches.
func (cur *MicroDepositCursor) Next() ([]UploadableMicroDeposit, error) {
	query := `select depository_id, user_id, amount, file_id, created_at from micro_deposits where deleted_at is null and merged_filename is null and created_at > ? order by created_at asc limit ?`
	stmt, err := cur.DepRepo.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("microDepositCursor.Next: prepare: %v", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(cur.newerThan, cur.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("microDepositCursor.Next: query: %v", err)
	}
	defer rows.Close()

	max := cur.newerThan
	var microDeposits []UploadableMicroDeposit
	for rows.Next() {
		var m UploadableMicroDeposit
		var amt string
		if err := rows.Scan(&m.DepositoryID, &m.UserID, &amt, &m.FileID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("transferCursor.Next: scan: %v", err)
		}
		var amount model.Amount
		if err := amount.FromString(amt); err != nil {
			return nil, fmt.Errorf("transferCursor.Next: %s Amount from string: %v", amt, err)
		}
		m.Amount = &amount
		if m.CreatedAt.After(max) {
			max = m.CreatedAt // advance to latest timestamp
		}
		microDeposits = append(microDeposits, m)
	}
	cur.newerThan = max
	return microDeposits, rows.Err()
}

// GetMicroDepositCursor returns a microDepositCursor for iterating through micro-deposits in ascending order (by CreatedAt)
// beginning at the start of the current day.
func (r *SQLRepo) GetMicroDepositCursor(batchSize int) *MicroDepositCursor {
	now := time.Now()
	return &MicroDepositCursor{
		BatchSize: batchSize,
		DepRepo:   r,
		newerThan: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
	}
}

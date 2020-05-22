// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"database/sql"
	"fmt"

	"github.com/moov-io/paygate/pkg/client"
)

type Repository interface {
	MarkTransfersAsProcessed(transferIDs []string) error
}

func NewRepo(db *sql.DB) *sqlRepo {
	return &sqlRepo{db: db}
}

type sqlRepo struct {
	db *sql.DB
}

// MarkTransfersAsProcessed updates the status for transfers and micro-deposits to PROCESSED.
// This signals that the underlying ACH file has been uploaded to the ODFI.
//
// It would be nicer to share this repository between ./pkg/transfers/ and
// ./pkg/validation/microdeposits/, but there are cyclic dependencies if it's put into either
// package.
func (r *sqlRepo) MarkTransfersAsProcessed(transferIDs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	transferQuery := `update transfers set status = ? where transfer_id = ? and deleted_at is null`
	transferStmt, err := tx.Prepare(transferQuery)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer transferStmt.Close()

	microQuery := `update micro_deposits set status = ? where micro_deposit_id = (
  select micro_deposit_id from micro_deposit_transfers where transfer_id = ?);`
	microStmt, err := tx.Prepare(microQuery)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer microStmt.Close()

	for i := range transferIDs {
		row, err := transferStmt.Exec(client.PROCESSED, transferIDs[i])
		if err != nil && err != sql.ErrNoRows {
			tx.Rollback()
			return err
		}
		if n, _ := row.RowsAffected(); n == 0 {
			tx.Rollback()
			return fmt.Errorf("transferID=%s not found / updated: %v", transferIDs[i], err)
		}

		// not every transfer is used in micro-deposits so we can ignore a zero row update
		_, err = microStmt.Exec(client.PROCESSED, transferIDs[i])
		if err != nil && err != sql.ErrNoRows {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

type Repository interface {
	getMicroDeposits(id id.Depository) ([]*Credit, error) // admin endpoint
	getMicroDepositsForUser(id id.Depository, userID id.User) ([]*Credit, error)

	InitiateMicroDeposits(id id.Depository, userID id.User, microDeposit []*Credit) error
	confirmMicroDeposits(id id.Depository, userID id.User, amounts []model.Amount) error

	LookupMicroDepositFromReturn(id id.Depository, amount *model.Amount) (*Credit, error)
	MarkMicroDepositAsMerged(filename string, mc UploadableCredit) error

	GetCursor(batchSize int) *Cursor
}

type SQLRepo struct {
	db     *sql.DB
	logger log.Logger
}

func NewRepository(logger log.Logger, db *sql.DB) *SQLRepo {
	return &SQLRepo{
		logger: logger,
		db:     db,
	}
}

// getMicroDeposits will retrieve the micro deposits for a given depository. This endpoint is designed for paygate's admin endpoints.
// If an amount does not parse it will be discardded silently.
func (r *SQLRepo) getMicroDeposits(id id.Depository) ([]*Credit, error) {
	query := `select amount, file_id, transaction_id from micro_deposits where depository_id = ?`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return accumulateMicroDeposits(rows)
}

// getMicroDepositsForUser will retrieve the micro deposits for a given depository. If an amount does not parse it will be discardded silently.
func (r *SQLRepo) getMicroDepositsForUser(id id.Depository, userID id.User) ([]*Credit, error) {
	query := `select amount, file_id, transaction_id from micro_deposits where user_id = ? and depository_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return accumulateMicroDeposits(rows)
}

// InitiateMicroDeposits will save the provided []Amount into our database. If amounts have already been saved then
// no new amounts will be added.
func (r *SQLRepo) InitiateMicroDeposits(id id.Depository, userID id.User, microDeposits []*Credit) error {
	existing, err := r.getMicroDepositsForUser(id, userID)
	if err != nil || len(existing) > 0 {
		return fmt.Errorf("not initializing more micro deposits, already have %d or got error=%v", len(existing), err)
	}

	// write amounts
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	now, query := time.Now(), `insert into micro_deposits (depository_id, user_id, amount, file_id, transaction_id, created_at) values (?, ?, ?, ?, ?, ?)`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("InitiateMicroDeposits: prepare error=%v rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()

	for i := range microDeposits {
		_, err = stmt.Exec(id, userID, microDeposits[i].Amount.String(), microDeposits[i].FileID, microDeposits[i].TransactionID, now)
		if err != nil {
			return fmt.Errorf("InitiateMicroDeposits: scan error=%v rollback=%v", err, tx.Rollback())
		}
	}

	return tx.Commit()
}

// confirmMicroDeposits will compare the provided guessAmounts against what's been persisted for a user. If the amounts do not match
// or there are a mismatched amount the call will return a non-nil error.
func (r *SQLRepo) confirmMicroDeposits(id id.Depository, userID id.User, guessAmounts []model.Amount) error {
	microDeposits, err := r.getMicroDepositsForUser(id, userID)
	if err != nil {
		return fmt.Errorf("unable to confirm micro deposits, got error=%v", err)
	}
	if len(microDeposits) == 0 {
		return errors.New("unable to confirm micro deposits, got 0 micro deposits")
	}

	// Check amounts, all must match
	if len(guessAmounts) != len(microDeposits) || len(guessAmounts) == 0 {
		return fmt.Errorf("incorrect amount of guesses, got %d", len(guessAmounts)) // don't share len(microDeposits), that's an info leak
	}

	found := 0
	for i := range microDeposits {
		for k := range guessAmounts {
			if microDeposits[i].Amount.Equal(guessAmounts[k]) {
				found += 1
				break
			}
		}
	}

	if found != len(microDeposits) {
		return errors.New("incorrect micro deposit guesses")
	}

	return nil
}

// MarkMicroDepositAsMerged will set the merged_filename on micro-deposits so they aren't merged into multiple files
// and the file uploaded to the Federal Reserve can be tracked.
func (r *SQLRepo) MarkMicroDepositAsMerged(filename string, mc UploadableCredit) error {
	query := `update micro_deposits set merged_filename = ?
where depository_id = ? and file_id = ? and amount = ? and (merged_filename is null or merged_filename = '') and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("MarkMicroDepositAsMerged: filename=%s: %v", filename, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(filename, mc.DepositoryID, mc.FileID, mc.Amount.String())
	return err
}

func (r *SQLRepo) LookupMicroDepositFromReturn(id id.Depository, amount *model.Amount) (*Credit, error) {
	query := `select file_id from micro_deposits where depository_id = ? and amount = ? and deleted_at is null order by created_at desc limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("LookupMicroDepositFromReturn prepare: %v", err)
	}
	defer stmt.Close()

	var fileID string
	if err := stmt.QueryRow(id, amount.String()).Scan(&fileID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("LookupMicroDepositFromReturn scan: %v", err)
	}
	if string(fileID) != "" {
		return &Credit{Amount: *amount, FileID: fileID}, nil
	}
	return nil, nil
}

func (r *SQLRepo) getMicroDepositReturnCodes(id id.Depository) []*ach.ReturnCode {
	query := `select distinct md.return_code from micro_deposits as md
inner join depositories as deps on md.depository_id = deps.depository_id
where md.depository_id = ? and deps.status = ? and md.return_code <> '' and md.deleted_at is null and deps.deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil
	}
	defer stmt.Close()

	rows, err := stmt.Query(id, model.DepositoryRejected)
	if err != nil {
		return nil
	}
	defer rows.Close()

	returnCodes := make(map[string]*ach.ReturnCode)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil
		}
		if _, exists := returnCodes[code]; !exists {
			returnCodes[code] = ach.LookupReturnCode(code)
		}
	}

	var codes []*ach.ReturnCode
	for k := range returnCodes {
		codes = append(codes, returnCodes[k])
	}
	return codes
}

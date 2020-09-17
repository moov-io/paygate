// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"database/sql"
	"fmt"

	"github.com/moov-io/paygate/pkg/client"
)

type Repository interface {
	// TODO(adam): lookup a micro-deposit from transferID, for return handling

	getMicroDeposits(microDepositID string) (*client.MicroDeposits, error)
	getAccountMicroDeposits(accountID string) (*client.MicroDeposits, error)
	writeMicroDeposits(micro *client.MicroDeposits) error
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

func (r *sqlRepo) getMicroDeposits(microDepositID string) (*client.MicroDeposits, error) {
	query := `select micro_deposit_id, destination_customer_id, destination_account_id, status, processed_at, created_at from micro_deposits
where micro_deposit_id = ? and deleted_at is null limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("micro-deposit prepare: %v", err)
	}
	defer stmt.Close()

	var micro client.MicroDeposits
	if err := stmt.QueryRow(microDepositID).Scan(
		&micro.MicroDepositID,
		&micro.Destination.CustomerID,
		&micro.Destination.AccountID,
		&micro.Status,
		&micro.ProcessedAt,
		&micro.Created,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("micro-deposit scan: %v", err)
	}

	micro.TransferIDs, err = r.getMicroDepositTransferIDs(microDepositID)
	if err != nil {
		return nil, err
	}

	// Read out the amounts
	query = `select amount_currency, amount_value from micro_deposit_amounts where micro_deposit_id = ?;`
	stmt, err = r.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("micro-deposit amounts prepare: %v", err)
	}
	rows, err := stmt.Query(microDepositID)
	if err != nil {
		return nil, fmt.Errorf("micro-deposit amounts query: %v", err)
	}
	for rows.Next() {
		var amt client.Amount
		if err := rows.Scan(&amt.Currency, &amt.Value); err != nil {
			return nil, fmt.Errorf("micro-deposit amount scan: %v", err)
		}
		micro.Amounts = append(micro.Amounts, amt)
	}

	return &micro, nil
}

func (r *sqlRepo) getMicroDepositTransferIDs(microDepositID string) ([]string, error) {
	query := `select transfer_id from micro_deposit_transfers where micro_deposit_id = ?;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(microDepositID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transferIDs []string
	for rows.Next() {
		var transferID string
		if err := rows.Scan(&transferID); err != nil {
			return nil, err
		}
		transferIDs = append(transferIDs, transferID)
	}
	return transferIDs, nil
}

func (r *sqlRepo) getAccountMicroDeposits(accountID string) (*client.MicroDeposits, error) {
	query := `select micro_deposit_id from micro_deposits where destination_account_id = ? and deleted_at is null limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var microDepositID string
	if err := stmt.QueryRow(accountID).Scan(&microDepositID); err != nil {
		return nil, err
	}
	return r.getMicroDeposits(microDepositID)
}

func (r *sqlRepo) writeMicroDeposits(micro *client.MicroDeposits) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	if err := r.writeMicroDeposit(tx, micro); err != nil {
		tx.Rollback()
		return fmt.Errorf("micro-deposits write: %v", err)
	}

	if err := r.writeMicroDepositAmounts(tx, micro.MicroDepositID, micro.Amounts); err != nil {
		tx.Rollback()
		return fmt.Errorf("micro-deposits write amounts: %v", err)
	}

	if err := r.writeMicroDepositTransferIDs(tx, micro.MicroDepositID, micro.TransferIDs); err != nil {
		tx.Rollback()
		return fmt.Errorf("micro-deposits: write transferIDs: %v", err)
	}

	return tx.Commit()
}

func (r *sqlRepo) writeMicroDeposit(tx *sql.Tx, micro *client.MicroDeposits) error {
	query := `insert into micro_deposits (micro_deposit_id, destination_customer_id, destination_account_id, status, created_at) values (?, ?, ?, ?, ?);`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(micro.MicroDepositID, micro.Destination.CustomerID, micro.Destination.AccountID, micro.Status, micro.Created)
	if err != nil {
		return err
	}
	return nil
}

func (r *sqlRepo) writeMicroDepositAmounts(tx *sql.Tx, microDepositID string, amounts []client.Amount) error {
	query := `insert into micro_deposit_amounts (micro_deposit_id, amount_currency, amount_value) values (?, ?, ?);`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range amounts {
		if _, err := stmt.Exec(microDepositID, amounts[i].Currency, amounts[i].Value); err != nil {
			return err
		}
	}
	return nil
}

func (r *sqlRepo) writeMicroDepositTransferIDs(tx *sql.Tx, microDepositID string, transferIDs []string) error {
	query := `insert into micro_deposit_transfers (micro_deposit_id, transfer_id) values (?, ?);`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range transferIDs {
		if _, err := stmt.Exec(microDepositID, transferIDs[i]); err != nil {
			return err
		}
	}
	return nil
}

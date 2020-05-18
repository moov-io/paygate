// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"database/sql"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
)

type Repository interface {
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
	query := `select micro_deposit_id, destination_customer_id, destination_account_id, amounts, status, return_code, created_at from micro_deposits
where micro_deposit_id = ? and deleted_at is null limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(microDepositID)

	var amounts string
	var returnCode *string

	var micro client.MicroDeposits
	if err := row.Scan(
		&micro.MicroDepositID,
		&micro.Destination.CustomerID,
		&micro.Destination.AccountID,
		&amounts,
		&micro.Status,
		&returnCode,
		&micro.Created,
	); err != nil {
		return nil, err
	}

	micro.Amounts = strings.Split(amounts, "|")
	if returnCode != nil {
		if rc := ach.LookupReturnCode(*returnCode); rc != nil {
			micro.ReturnCode = client.ReturnCode{
				Code:        rc.Code,
				Reason:      rc.Reason,
				Description: rc.Description,
			}
		}
	}

	return &micro, nil
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
	query := `insert into micro_deposits (micro_deposit_id, destination_customer_id, destination_account_id, amounts, status, return_code, created_at) values (?, ?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		micro.MicroDepositID,
		micro.Destination.CustomerID,
		micro.Destination.AccountID,
		strings.Join(micro.Amounts, "|"),
		micro.Status,
		micro.ReturnCode.Code,
		micro.Created,
	)
	return err
}

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
)

type Attemper struct {
	db     *sql.DB
	logger log.Logger

	maxAttempts int
}

func NewAttemper(logger log.Logger, db *sql.DB, maxAttempts int) *Attemper {
	return &Attemper{db: db, logger: logger, maxAttempts: maxAttempts}
}

func (at *Attemper) Available(id DepositoryID) bool {
	query := `select count(*) from micro_deposit_attempts where depository_id = ?;`
	stmt, err := at.db.Prepare(query)
	if err != nil {
		at.logger.Log("micro-deposits", fmt.Sprintf("error getting attempts: %v", err))
		return false
	}
	var count int
	if err := stmt.QueryRow(string(id)).Scan(&count); err != nil {
		at.logger.Log("micro-deposits", fmt.Sprintf("problem scanning attempts: %v", err))
		return false
	}
	return count < at.maxAttempts
}

func (at *Attemper) Record(id DepositoryID, amounts string) error {
	query := `insert into micro_deposit_attempts (depository_id, amounts, attempted_at) values (?, ?, ?);`
	stmt, err := at.db.Prepare(query)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(string(id), amounts, time.Now())
	return err
}

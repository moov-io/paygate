// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/paygate/pkg/id"
)

type attempter interface {
	Available(id id.Depository) bool

	// Record will track the list of amounts (comma separated string) for human debugging
	Record(id id.Depository, amounts string) error
}

func NewAttemper(logger log.Logger, db *sql.DB, maxAttempts int) attempter {
	return &sqlAttempter{db: db, logger: logger, maxAttempts: maxAttempts}
}

type sqlAttempter struct {
	db     *sql.DB
	logger log.Logger

	maxAttempts int
}

func (at *sqlAttempter) Available(id id.Depository) bool {
	query := `select count(*) from micro_deposit_attempts where depository_id = ?;`
	stmt, err := at.db.Prepare(query)
	if err != nil {
		at.logger.Log("micro-deposits", fmt.Sprintf("error getting attempts: %v", err))
		return false
	}
	defer stmt.Close()

	var count int
	if err := stmt.QueryRow(string(id)).Scan(&count); err != nil {
		at.logger.Log("micro-deposits", fmt.Sprintf("problem scanning attempts: %v", err))
		return false
	}
	return count < at.maxAttempts
}

func (at *sqlAttempter) Record(id id.Depository, amounts string) error {
	query := `insert into micro_deposit_attempts (depository_id, amounts, attempted_at) values (?, ?, ?);`
	stmt, err := at.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(string(id), amounts, time.Now())
	return err
}

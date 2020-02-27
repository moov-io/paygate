// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

// OneDayLimit returns the maximum sum of transfers for each user over the current day.
func OneDayLimit() string {
	if v := os.Getenv("TRANSFERS_ONE_DAY_LIMIT"); v != "" {
		return v
	}
	return "5000.00"
}

// SevenDayLimit returns the maximum sum of transfers for each user over the previous seven days.
func SevenDayLimit() string {
	if v := os.Getenv("TRANSFERS_SEVEN_DAY_LIMIT"); v != "" {
		return v
	}
	return "10000.00"
}

// ThirtyDayLimit returns the maximum sum of transfers for each user over the previous thirty days.
func ThirtyDayLimit() string {
	if v := os.Getenv("TRANSFERS_THIRTY_DAY_LIMIT"); v != "" {
		return v
	}
	return "25000.00"
}

// ParseLimits attempts to convert multiple strings into Amount objects.
// These need to follow the Amount format (e.g. 10000.00)
func ParseLimits(oneDay, sevenDays, thirtyDays string) (*Limits, error) {
	one, err := model.NewAmount("USD", oneDay)
	if err != nil {
		return nil, err
	}
	seven, err := model.NewAmount("USD", sevenDays)
	if err != nil {
		return nil, err
	}
	thirty, err := model.NewAmount("USD", thirtyDays)
	if err != nil {
		return nil, err
	}
	return &Limits{
		CurrentDay:        one,
		PreviousSevenDays: seven,
		PreviousThityDays: thirty,
	}, nil
}

// Limits contain the maximum Amount transfers can accumulate to over a given time period.
type Limits struct {
	CurrentDay        *model.Amount
	PreviousSevenDays *model.Amount
	PreviousThityDays *model.Amount
}

// NewLimitChecker returns a LimitChecker instance to sum transfers for a userID or routing number.
func NewLimitChecker(logger log.Logger, db *sql.DB, limits *Limits) *LimitChecker {
	lc := &LimitChecker{
		logger: logger,
		db:     db,
		limits: limits,
	}

	switch strings.ToLower(database.Type()) {
	case "sqlite":
		lc.userTransferSumSQL = sqliteSumUserTransfers
		lc.routingNumberTransferSumSQL = sqliteSumTransfersByRoutingNumber

	case "mysql":
		lc.userTransferSumSQL = mysqlSumUserTransfers
		lc.routingNumberTransferSumSQL = mysqlSumTransfersByRoutingNumber
	}

	return lc
}

// LimitChecker is an instance which accumulates transfers for a given userID or routing number to
// verify if a pending transfer should be accepted according to how much money is allowed to transfer
// over a given time period.
type LimitChecker struct {
	db     *sql.DB
	logger log.Logger

	limits *Limits

	userTransferSumSQL          string // must require ordered user_id, created_at parameters
	routingNumberTransferSumSQL string // must require ordered routing_number, created_at parameters
}

var (
	// SQLite queries to sum transfers
	sqliteSumUserTransfers = `select sum(trim(amount, "USD ")) from transfers
where user_id = ? and created_at > ? and deleted_at is null;`

	sqliteSumTransfersByRoutingNumber = `select sum(trim(transfers.amount, "USD ")) from transfers
inner join depositories on transfers.receiver_depository = depositories.depository_id
where depositories.routing_number = ? and transfers.created_at > ?
and transfers.deleted_at is null and depositories.deleted_at is null;`

	// MySQL queries to sum transfers
	mysqlSumUserTransfers = `select sum(trim(LEADING "USD " FROM amount)) from transfers
where user_id = ? and created_at > ? and deleted_at is null;`

	mysqlSumTransfersByRoutingNumber = `select sum(trim(LEADING "USD " FROM amount)) from transfers
inner join depositories on transfers.receiver_depository = depositories.depository_id
where depositories.routing_number = ? and transfers.created_at > ?
and transfers.deleted_at is null and depositories.deleted_at is null;`
)

func overLimit(total float64, max *model.Amount) error {
	if total < 0.00 {
		return errors.New("invalid total")
	}
	if int(total*100) >= max.Int() {
		return errors.New("over limit")
	}
	return nil
}

func (lc *LimitChecker) allowTransfer(userID id.User, routingNumber string) error {
	if err := lc.previousDasUnderLimit(userID, routingNumber); err != nil {
		return err
	}
	if err := lc.previousSevenDaysUnderLimit(userID, routingNumber); err != nil {
		return err
	}
	if err := lc.previousThirtyDaysUnderLimit(userID, routingNumber); err != nil {
		return err
	}
	return nil
}

func (lc *LimitChecker) previousDasUnderLimit(userID id.User, routingNumber string) error {
	currentDay := time.Now().UTC().Add(-24 * time.Hour).Truncate(24 * time.Hour)
	return lc.underLimits(userID, routingNumber, lc.limits.CurrentDay, currentDay)
}

func (lc *LimitChecker) previousSevenDaysUnderLimit(userID id.User, routingNumber string) error {
	sevenDaysAgo := time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(24 * time.Hour)
	return lc.underLimits(userID, routingNumber, lc.limits.PreviousSevenDays, sevenDaysAgo)
}

func (lc *LimitChecker) previousThirtyDaysUnderLimit(userID id.User, routingNumber string) error {
	thirtyDaysAgo := time.Now().UTC().Add(-30 * 24 * time.Hour).Truncate(24 * time.Hour)
	return lc.underLimits(userID, routingNumber, lc.limits.PreviousThityDays, thirtyDaysAgo)
}

func (lc *LimitChecker) underLimits(userID id.User, routingNumber string, limit *model.Amount, newerThan time.Time) error {
	daysAgo := int(time.Since(newerThan).Hours() / 24)

	total, err := lc.userTransferSum(userID, newerThan)
	if err != nil {
		return fmt.Errorf("limits: error getting %d day user total: %v", daysAgo, err)
	}
	if err := overLimit(total, limit); err != nil {
		return fmt.Errorf("limits: previous %d days of user transfers would be over: %v", daysAgo, err)
	}

	total, err = lc.routingNumberSum(routingNumber, newerThan)
	if err != nil {
		return fmt.Errorf("limits: error getting %d day routing number total: %v", daysAgo, err)
	}
	if err := overLimit(total, limit); err != nil {
		return fmt.Errorf("limits: previous %d days of transfers for routing number would be over: %v", daysAgo, err)
	}

	return nil
}

func (lc *LimitChecker) userTransferSum(userID id.User, newerThan time.Time) (float64, error) {
	stmt, err := lc.db.Prepare(lc.userTransferSumSQL)
	if err != nil {
		return -1.0, fmt.Errorf("user transfers prepare: %v", err)
	}
	defer stmt.Close()

	var total *float64
	if err := stmt.QueryRow(userID, newerThan).Scan(&total); err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("A")
			return 0.0, nil
		}
		return -1.0, fmt.Errorf("user transfers query: %v", err)
	}
	if total != nil {
		return *total, nil
	}
	return 0.0, nil
}

func (lc *LimitChecker) routingNumberSum(routingNumber string, newerThan time.Time) (float64, error) {
	stmt, err := lc.db.Prepare(lc.routingNumberTransferSumSQL)
	if err != nil {
		return -1.0, fmt.Errorf("routing numbers transfers prepare: %v", err)
	}
	defer stmt.Close()

	var total *float64
	if err := stmt.QueryRow(routingNumber, newerThan).Scan(&total); err != nil {
		if err == sql.ErrNoRows {
			return 0.0, nil
		}
		return -1.0, fmt.Errorf("routing numbers transfers query: %v", err)
	}
	if total != nil {
		return *total, nil
	}
	return 0.0, nil
}

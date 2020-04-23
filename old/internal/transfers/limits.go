// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/util"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

// OneDayLimit returns the maximum sum of transfers for each user over the current day.
func OneDayLimit() string {
	return util.Or(os.Getenv("TRANSFERS_ONE_DAY_USER_LIMIT"), "5000.00")
}

// SevenDayLimit returns the maximum sum of transfers for each user over the previous seven days.
func SevenDayLimit() string {
	return util.Or(os.Getenv("TRANSFERS_SEVEN_DAY_USER_LIMIT"), "10000.00")
}

// ThirtyDayLimit returns the maximum sum of transfers for each user over the previous thirty days.
func ThirtyDayLimit() string {
	return util.Or(os.Getenv("TRANSFERS_THIRTY_DAY_USER_LIMIT"), "25000.00")
}

// ParseLimits attempts to convert multiple strings into Amount objects.
// These need to follow the Amount format (e.g. 10000.00)
func ParseLimits(oneDay, sevenDays, thirtyDays string) (*Limits, error) {
	one, err := model.NewAmount("USD", oneDay)
	if err != nil {
		return nil, fmt.Errorf("one day: %v", err)
	}
	seven, err := model.NewAmount("USD", sevenDays)
	if err != nil {
		return nil, fmt.Errorf("seven day: %v", err)
	}
	thirty, err := model.NewAmount("USD", thirtyDays)
	if err != nil {
		return nil, fmt.Errorf("thirty day: %v", err)
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

// NewLimitChecker returns a LimitChecker instance to sum transfers for a userID .
func NewLimitChecker(logger log.Logger, db *sql.DB, limits *Limits) *LimitChecker {
	lc := &LimitChecker{
		logger: logger,
		db:     db,
		limits: limits,
	}

	switch strings.ToLower(database.Type()) {
	case "sqlite":
		lc.userTransferSumSQL = sqliteSumUserTransfers

	case "mysql":
		lc.userTransferSumSQL = mysqlSumUserTransfers
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

	userTransferSumSQL string // must require ordered user_id, created_at parameters
}

var (
	sqliteSumUserTransfers = `select sum(trim(amount, "USD ")) from transfers
where user_id = ? and created_at > ? and deleted_at is null;`

	mysqlSumUserTransfers = `select sum(trim(LEADING "USD " FROM amount)) from transfers
where user_id = ? and created_at > ? and deleted_at is null;`
)

var (
	errOverLimit = errors.New("transfers over limit")
)

func overLimit(total float64, max *model.Amount) error {
	if total < 0.00 {
		return errors.New("invalid total")
	}
	if int(total*100) >= max.Int() {
		return errOverLimit
	}
	return nil
}

func (lc *LimitChecker) allowTransfer(userID id.User) error {
	if err := lc.previousDasUnderLimit(userID); err != nil {
		return err
	}
	if err := lc.previousSevenDaysUnderLimit(userID); err != nil {
		return err
	}
	if err := lc.previousThirtyDaysUnderLimit(userID); err != nil {
		return err
	}
	return nil
}

func (lc *LimitChecker) previousDasUnderLimit(userID id.User) error {
	currentDay := time.Now().UTC().Add(-24 * time.Hour).Truncate(24 * time.Hour)
	return lc.underLimits(userID, lc.limits.CurrentDay, currentDay)
}

func (lc *LimitChecker) previousSevenDaysUnderLimit(userID id.User) error {
	sevenDaysAgo := time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(24 * time.Hour)
	return lc.underLimits(userID, lc.limits.PreviousSevenDays, sevenDaysAgo)
}

func (lc *LimitChecker) previousThirtyDaysUnderLimit(userID id.User) error {
	thirtyDaysAgo := time.Now().UTC().Add(-30 * 24 * time.Hour).Truncate(24 * time.Hour)
	return lc.underLimits(userID, lc.limits.PreviousThityDays, thirtyDaysAgo)
}

func (lc *LimitChecker) underLimits(userID id.User, limit *model.Amount, newerThan time.Time) error {
	daysAgo := int(time.Since(newerThan).Hours() / 24)

	total, err := lc.userTransferSum(userID, newerThan)
	if err != nil {
		return fmt.Errorf("limits: error getting %d day user total: %v", daysAgo, err)
	}
	if err := overLimit(total, limit); err != nil {
		return fmt.Errorf("limits: previous %d days of user transfers would be over: %v", daysAgo, err)
	}

	return nil
}

func (lc *LimitChecker) userTransferSum(userID id.User, newerThan time.Time) (float64, error) {
	stmt, err := lc.db.Prepare(lc.userTransferSumSQL)
	if err != nil {
		return -1.0, fmt.Errorf("user transfers prepare: %v", err)
	}
	defer stmt.Close()

	var total *string
	if err := stmt.QueryRow(userID, newerThan).Scan(&total); err != nil {
		if err == sql.ErrNoRows {
			return 0.0, nil
		}
		return -1.0, fmt.Errorf("user transfers query: %v", err)
	}
	if total != nil {
		f, _ := strconv.ParseFloat(*total, 64)
		return f, nil
	}
	return 0.0, nil
}

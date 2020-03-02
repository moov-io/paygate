// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

func TestLimits__ParseLimits(t *testing.T) {
	if limits, err := ParseLimits(OneDayLimit(), SevenDayLimit(), ThirtyDayLimit()); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else {
		if limits.CurrentDay.Int() != 5000*100 {
			t.Errorf("got %v", limits.CurrentDay)
		}
		if limits.PreviousSevenDays.Int() != 10000*100 {
			t.Errorf("got %v", limits.PreviousSevenDays)
		}
		if limits.PreviousThityDays.Int() != 25000*100 {
			t.Errorf("got %v", limits.PreviousThityDays)
		}
	}

	if limits, err := ParseLimits("100.00", "1000.00", "123456.00"); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else {
		if limits.CurrentDay.Int() != 100*100 {
			t.Errorf("got %v", limits.CurrentDay)
		}
		if limits.PreviousSevenDays.Int() != 1000*100 {
			t.Errorf("got %v", limits.PreviousSevenDays)
		}
		if limits.PreviousThityDays.Int() != 123456*100 {
			t.Errorf("got %v", limits.PreviousThityDays)
		}
	}

	if limits, err := ParseLimits("1.00", "10.00", "1.21"); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else {
		if limits.CurrentDay.Int() != 1*100 {
			t.Errorf("got %v", limits.CurrentDay)
		}
		if limits.PreviousSevenDays.Int() != 10*100 {
			t.Errorf("got %v", limits.PreviousSevenDays)
		}
		if limits.PreviousThityDays.Int() != 121 {
			t.Errorf("got %v", limits.PreviousThityDays)
		}
	}
}

func TestLimits__ParseLimitsErr(t *testing.T) {
	if l, err := ParseLimits(OneDayLimit(), SevenDayLimit(), "invalid"); err == nil {
		t.Logf("%v", l)
		t.Error("expected error")
	}
	if l, err := ParseLimits("invalid", SevenDayLimit(), ThirtyDayLimit()); err == nil {
		t.Logf("%v", l)
		t.Error("expected error")
	}
	if l, err := ParseLimits(OneDayLimit(), "invalid", ThirtyDayLimit()); err == nil {
		t.Logf("%v", l)
		t.Error("expected error")
	}
}

func TestLimits__overLimit(t *testing.T) {
	if err := overLimit(-1.0, nil); err == nil {
		t.Error("expected error")
	}
}

func TestLimits__integration(t *testing.T) {
	t.Parallel()

	limits, err := ParseLimits("100.00", "100.00", "250.00")
	if err != nil {
		t.Fatal(err)
	}

	check := func(t *testing.T, lc *LimitChecker) {
		userID := id.User(base.ID())

		// no transfers yet
		if err := lc.allowTransfer(userID); err != nil {
			t.Fatal(err)
		}

		// write a transfer
		amt, _ := model.NewAmount("USD", "25.12")
		xferReq := []*transferRequest{
			{
				Type:                   model.PushTransfer,
				Amount:                 *amt,
				Originator:             model.OriginatorID("originator"),
				OriginatorDepository:   id.Depository("originator"),
				Receiver:               model.ReceiverID("receiver"),
				ReceiverDepository:     id.Depository("receiver"),
				Description:            "money",
				StandardEntryClassCode: "PPD",
				fileID:                 "test-file",
			},
		}

		repo := NewTransferRepo(log.NewNopLogger(), lc.db)
		if _, err = repo.createUserTransfers(userID, xferReq); err != nil {
			t.Fatal(err)
		}

		if total, err := lc.userTransferSum(userID, time.Now().Add(-24*time.Hour)); err != nil {
			t.Fatal(err)
		} else {
			if int(total*100) != amt.Int() {
				t.Errorf("got %.2f", total)
			}
		}

		// write another transfer
		amt, _ = model.NewAmount("USD", "121.44")
		xferReq[0].Amount = *amt
		if _, err := repo.createUserTransfers(userID, xferReq); err != nil {
			t.Fatal(err)
		}

		// ensure it's blocked
		if err := lc.allowTransfer(userID); err == nil {
			t.Fatal("expected error")
		}
		if total, err := lc.userTransferSum(userID, time.Now().Add(-24*time.Hour)); err != nil {
			t.Fatal(err)
		} else {
			if int(total*100) != 2512+12144 {
				t.Errorf("got %.2f", total)
			}
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	lc := NewLimitChecker(log.NewNopLogger(), sqliteDB.DB, limits)
	check(t, lc)

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()

	lc = NewLimitChecker(log.NewNopLogger(), mysqlDB.DB, limits)
	lc.userTransferSumSQL = mysqlSumUserTransfers
	check(t, lc)
}

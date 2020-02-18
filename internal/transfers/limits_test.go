// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal"
)

func TestLimits__ParseLimits(t *testing.T) {
	if limits, err := ParseLimits(sevenDayLimit, thirtyDayLimit); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else {
		if limits.PreviousSevenDays.Int() != 10000*100 {
			t.Errorf("got %v", limits.PreviousSevenDays)
		}
		if limits.PreviousThityDays.Int() != 25000*100 {
			t.Errorf("got %v", limits.PreviousThityDays)
		}
	}

	if limits, err := ParseLimits("1000.00", "123456.00"); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else {
		if limits.PreviousSevenDays.Int() != 1000*100 {
			t.Errorf("got %v", limits.PreviousSevenDays)
		}
		if limits.PreviousThityDays.Int() != 123456*100 {
			t.Errorf("got %v", limits.PreviousThityDays)
		}
	}

	if limits, err := ParseLimits("10.00", thirtyDayLimit); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else {
		if limits.PreviousSevenDays.Int() != 10*100 {
			t.Errorf("got %v", limits.PreviousSevenDays)
		}
		if limits.PreviousThityDays.Int() != 25000*100 {
			t.Errorf("got %v", limits.PreviousThityDays)
		}
	}
}

func TestLimits__UnderLimits(t *testing.T) {
	amt, _ := internal.NewAmount("USD", "100.00")
	xfer := &internal.Transfer{
		Amount:  *amt,
		Created: base.NewTime(time.Now()),
	}

	seven, _ := internal.NewAmount("USD", "500.00")
	thirty, _ := internal.NewAmount("USD", "750.00")
	limits := &Limits{
		PreviousSevenDays: seven,
		PreviousThityDays: thirty,
	}

	if err := UnderLimits(nil, xfer, limits); err != nil {
		t.Fatal(err)
	}

	old, _ := internal.NewAmount("USD", "450.00")
	existing := []*internal.Transfer{
		{
			Amount:  *old,
			Created: base.NewTime(time.Now()),
		},
	}
	if err := UnderLimits(existing, xfer, limits); err == nil {
		t.Error("expected error")
	}

	old2, _ := internal.NewAmount("USD", "250.00")
	existing = append(existing, &internal.Transfer{
		Amount:  *old2,
		Created: base.NewTime(time.Now().Add(-10 * 24 * time.Hour)), // 10 days ago
	})
	if err := UnderLimits(existing, xfer, limits); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "over limit by USD 50.00") {
			t.Errorf("unexpected error: %v", err)
		}
	}

	exact, err := internal.NewAmountFromInt("USD", seven.Int()-old.Int())
	if err != nil {
		t.Fatal(err)
	}
	xfer.Amount = *exact
	if err := UnderLimits(existing, xfer, limits); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "over limit by USD 0.00") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

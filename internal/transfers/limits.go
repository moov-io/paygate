// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"fmt"
	"os"
	"time"

	"github.com/moov-io/paygate/internal"
)

var (
	sevenDayLimit = func() string {
		if v := os.Getenv("TRANSFERS_SEVEN_DAY_SOFT_LIMIT"); v != "" {
			return v
		}
		return "10000.00"
	}()
	thirtyDayLimit = func() string {
		if v := os.Getenv("TRANSFERS_THIRTY_DAY_SOFT_LIMIT"); v != "" {
			return v
		}
		return "25000.00"
	}()
)

// ParseLimits attempts to convert multiple strings into Amount objects.
// These need to follow the Amount format (e.g. 10000.00)
func ParseLimits(sevenDays, thirtyDays string) (*Limits, error) {
	seven, err := internal.NewAmount("USD", sevenDays)
	if err != nil {
		return nil, err
	}
	thirty, err := internal.NewAmount("USD", thirtyDays)
	if err != nil {
		return nil, err
	}
	return &Limits{
		PreviousSevenDays: seven,
		PreviousThityDays: thirty,
	}, nil
}

// Limits contain the maximum Amount transfers can accumulate to over a given time period.
type Limits struct {
	PreviousSevenDays *internal.Amount
	PreviousThityDays *internal.Amount
}

// UnderLimits checks if the set of existing transfers combined with a pending transfer would be over
// any defined limits.
func UnderLimits(existing []*internal.Transfer, pending *internal.Transfer, limits *Limits) error {
	if len(existing) == 0 {
		return nil
	}
	if limits.PreviousSevenDays != nil {
		if err := previousSevenDaysUnderLimit(existing, pending, limits.PreviousSevenDays); err != nil {
			return err
		}
	}
	if limits.PreviousThityDays != nil {
		if err := previousThityDaysUnderLimit(existing, pending, limits.PreviousThityDays); err != nil {
			return err
		}
	}
	return nil
}

func previousSevenDaysUnderLimit(existing []*internal.Transfer, pending *internal.Transfer, limit *internal.Amount) error {
	newerThan := time.Now().Add(-7 * 24 * time.Hour).Truncate(24 * time.Hour)
	return previousDaysUnderLimit(existing, pending, limit, newerThan)
}

func previousThityDaysUnderLimit(existing []*internal.Transfer, pending *internal.Transfer, limit *internal.Amount) error {
	newerThan := time.Now().Add(-30 * 24 * time.Hour).Truncate(24 * time.Hour)
	return previousDaysUnderLimit(existing, pending, limit, newerThan)
}

func previousDaysUnderLimit(existing []*internal.Transfer, pending *internal.Transfer, limit *internal.Amount, newerThan time.Time) error {
	total, err := sumTransfers(existing, newerThan).Plus(pending.Amount)
	if err != nil {
		return fmt.Errorf("limits: total error: %v", err)
	}

	if total.Int() >= limit.Int() {
		amt, err := internal.NewAmountFromInt("USD", total.Int()-limit.Int())
		if err != nil {
			return fmt.Errorf("limits: error=%v", err)
		}
		return fmt.Errorf("previous seven days would transfer %v over limit", amt)
	}
	return nil
}

func sumTransfers(existing []*internal.Transfer, newerThan time.Time) *internal.Amount {
	sum, _ := internal.NewAmount("USD", "0.00")
	for i := range existing {
		if existing[i].Created.After(newerThan) {
			sum.Plus(existing[i].Amount)
		}
	}
	return sum
}

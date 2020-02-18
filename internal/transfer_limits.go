// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"errors"
	"fmt"
	"os"
	"time"
)

var (
	SevenDayLimit = func() string {
		if v := os.Getenv("TRANSFERS_SEVEN_DAY_SOFT_LIMIT"); v != "" {
			return v
		}
		return "10000.00"
	}()
	ThirtyDayLimit = func() string {
		if v := os.Getenv("TRANSFERS_THIRTY_DAY_SOFT_LIMIT"); v != "" {
			return v
		}
		return "25000.00"
	}()
)

// ParseLimits attempts to convert multiple strings into Amount objects.
// These need to follow the Amount format (e.g. 10000.00)
func ParseLimits(sevenDays, thirtyDays string) (*Limits, error) {
	seven, err := NewAmount("USD", sevenDays)
	if err != nil {
		return nil, err
	}
	thirty, err := NewAmount("USD", thirtyDays)
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
	PreviousSevenDays *Amount
	PreviousThityDays *Amount
}

// UnderLimits checks if the set of existing transfers combined with a pending transfer would be over
// any defined limits.
func UnderLimits(existing []*Transfer, pending *Amount, limits *Limits) error {
	if limits == nil {
		return errors.New("missing Limits")
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

func previousSevenDaysUnderLimit(existing []*Transfer, pending *Amount, limit *Amount) error {
	newerThan := time.Now().Add(-7 * 24 * time.Hour).Truncate(24 * time.Hour)
	return previousDaysUnderLimit(existing, pending, limit, newerThan)
}

func previousThityDaysUnderLimit(existing []*Transfer, pending *Amount, limit *Amount) error {
	newerThan := time.Now().Add(-30 * 24 * time.Hour).Truncate(24 * time.Hour)
	return previousDaysUnderLimit(existing, pending, limit, newerThan)
}

func previousDaysUnderLimit(existing []*Transfer, pending *Amount, limit *Amount, newerThan time.Time) error {
	total, err := sumTransfers(existing, newerThan).Plus(*pending)
	if err != nil {
		return fmt.Errorf("limits: total error: %v", err)
	}

	if total.Int() >= limit.Int() {
		amt, err := NewAmountFromInt("USD", total.Int()-limit.Int())
		if err != nil {
			return fmt.Errorf("limits: error=%v", err)
		}
		return fmt.Errorf("existing and pending transfers would be over limit by %v", amt)
	}
	return nil
}

func sumTransfers(existing []*Transfer, newerThan time.Time) *Amount {
	sum, _ := NewAmount("USD", "0.00")
	for i := range existing {
		if existing[i].Created.After(newerThan) {
			s, _ := sum.Plus(existing[i].Amount)
			sum = &s
		}
	}
	return sum
}

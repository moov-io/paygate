// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"time"
)

// StartOfDayAndTomorrow returns two time.Time values from a given time.Time value.
// The first is at the start of the same day as provided and the second is exactly 24 hours
// after the first.
func StartOfDayAndTomorrow(in time.Time) (time.Time, time.Time) {
	start := in.Truncate(24 * time.Hour)
	return start, start.Add(24 * time.Hour)
}

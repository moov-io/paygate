// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package util

import (
	"time"
)

const (
	YYMMDDTimeFormat = "2006-01-02"
)

// FirstParsedTime attempts to parse v with all provided formats in order.
// The first parsed time.Time that doesn't error is returned.
func FirstParsedTime(v string, formats ...string) time.Time {
	for i := range formats {
		if tt, err := time.Parse(formats[i], v); err == nil {
			return tt
		}
	}
	return time.Time{}
}

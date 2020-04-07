// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package route

import (
	"math"
	"net/http"
	"strconv"
)

const (
	maxLimit int64 = 1000
)

// ReadOffset returns the "offset" query param from a request or zero if it's missing.
func ReadOffset(r *http.Request) int64 {
	return readIntQueryParam(r, "offset", math.MaxInt64)
}

// ReadLimit returns the "limit" query param from a request or zero if it's missing.
func ReadLimit(r *http.Request) int64 {
	return readIntQueryParam(r, "limit", maxLimit)
}

func readIntQueryParam(r *http.Request, key string, max int64) int64 {
	if v := r.URL.Query().Get(key); v != "" {
		limit, _ := strconv.ParseInt(v, 10, 32)
		if limit > max {
			return max
		}
		return limit
	}
	return 0
}

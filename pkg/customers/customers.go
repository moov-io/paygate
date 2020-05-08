// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"net/http"
	"time"
)

var (
	HttpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
)

// OfacSearch represents a search performed against OFAC data
type OfacSearch struct {
	EntityId  string
	SdnName   string
	Match     float32
	CreatedAt time.Time
}

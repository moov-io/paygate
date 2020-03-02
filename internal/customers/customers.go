// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"time"
)

// Customer represents a human who can be referenced in transfers
type Customer struct {
	ID     string
	Status string
}

// Disclaimer is a section of text whose acceptance is required prior
// to transfer processing.
type Disclaimer struct {
	ID         string
	Text       string
	AcceptedAt time.Time
}

// OfacSearch represents a search performed against OFAC data
type OfacSearch struct {
	EntityId  string
	SdnName   string
	Match     float32
	CreatedAt time.Time
}

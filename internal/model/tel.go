// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

type TELDetail struct {
	PhoneNumber string         `json:"phoneNumber"`
	PaymentType TELPaymentType `json:"paymentType,omitempty"`
}

type TELPaymentType string

const (
	TELSingle      TELPaymentType = "single"
	TELReoccurring TELPaymentType = "reoccurring"
)

func (t TELPaymentType) Validate() error {
	switch t {
	case TELSingle, TELReoccurring:
		return nil
	default:
		return fmt.Errorf("TELPaymentType(%s) is invalid", t)
	}
}

func (t *TELPaymentType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = TELPaymentType(strings.ToLower(s))
	if err := t.Validate(); err != nil {
		return err
	}
	return nil
}

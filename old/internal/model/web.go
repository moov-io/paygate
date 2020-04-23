// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

type WEBDetail struct {
	PaymentInformation string         `json:"paymentInformation,omitempty"`
	PaymentType        WEBPaymentType `json:"paymentType,omitempty"`
}

type WEBPaymentType string

const (
	WEBSingle      WEBPaymentType = "single"
	WEBReoccurring WEBPaymentType = "reoccurring"
)

func (t WEBPaymentType) Validate() error {
	switch t {
	case WEBSingle, WEBReoccurring:
		return nil
	default:
		return fmt.Errorf("WEBPaymentType(%s) is invalid", t)
	}
}

func (t *WEBPaymentType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = WEBPaymentType(strings.ToLower(s))
	if err := t.Validate(); err != nil {
		return err
	}
	return nil
}

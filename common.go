// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/text/currency"
)

type AccountType string

const (
	Checking AccountType = "checking"
	Savings  AccountType = "savings"
)

func (t AccountType) empty() bool {
	return string(t) == ""
}

func (t AccountType) validate() error {
	switch t {
	case Checking, Savings:
		return nil
	default:
		return fmt.Errorf("AccountType(%s) is invalid", t)
	}
}

func (t *AccountType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = AccountType(strings.ToLower(s))
	if err := t.validate(); err != nil {
		return err
	}
	return nil
}

// Amount represents units of a particular currency.
type Amount struct {
	number *big.Rat
	symbol string // ISO 4217, i.e. USD, GBP
}

// Int returns the currency amount as an integer.
// Example: "USD 1.11" returns 111
func (a *Amount) Int() int {
	n, _ := a.number.Float64()
	return int(n * 100)
}

func (a *Amount) Validate() error {
	_, err := currency.ParseISO(a.symbol)
	return err
}

// NewAmount returns an Amount object after validating the ISO 4217 currency symbol.
func NewAmount(symbol string, number string) (*Amount, error) {
	sym, err := currency.ParseISO(symbol)
	if err != nil {
		return nil, err
	}

	n := new(big.Rat)
	n.SetString(number)
	return &Amount{n, sym.String()}, nil
}

// String returns an amount formatted with the currency.
// Examples:
//   USD 12.53
//   GBP 4.02
//
// The symbol returned corresponds to the ISO 4217 standard.
// Only one period used to signify decimal value will be included.
func (a *Amount) String() string {
	if a == nil || a.symbol == "" || a.number == nil {
		return ""
	}
	return fmt.Sprintf("%s %s", a.symbol, a.number.FloatString(2))
}

// FromString attempts to parse str as a valid currency symbol and
// the quantity.
// Examples:
//   USD 12.53
//   GBP 4.02
func (a *Amount) FromString(str string) error {
	parts := strings.Fields(str)
	if len(parts) != 2 {
		return fmt.Errorf("invalid Amount format: %q", str)
	}

	sym, err := currency.ParseISO(parts[0])
	if err != nil {
		return err
	}

	number := new(big.Rat)
	number.SetString(parts[1])

	a.number = number
	a.symbol = sym.String()
	return nil
}

func (a Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *Amount) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return a.FromString(s)
}

// nextID creates a new ID for our system.
// Do no assume anything about these ID's other than
// they are strings. Case matters!
func nextID() string {
	bs := make([]byte, 20)
	n, err := rand.Read(bs)
	if err != nil || n == 0 {
		logger.Log("generateID", fmt.Sprintf("n=%d, err=%v", n, err))
		return ""
	}
	return strings.ToLower(hex.EncodeToString(bs))
}

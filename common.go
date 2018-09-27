// Copyright 2018 The Paygate Authors
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
	Checking AccountType = "Checking"
	Savings              = "Savings"
)

func (t *AccountType) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("AccountType must be a quoted string")
	}

	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "checking":
		*t = Checking
		return nil
	case "savings":
		*t = Savings
		return nil
	}
	return fmt.Errorf("unknown AccountType %q", s)
}

// Amount represents units of a particular currency.
type Amount struct {
	number *big.Rat
	symbol string // ISO 4217, i.e. USD, GBP
}

// String returns an amount formatted with the currency.
// Examples:
//   USD 12.53
//   GBP 4.02
//
// The symbol returned corresponds to the ISO 4217 standard.
func (a *Amount) String() string {
	return fmt.Sprintf("%s %s", a.symbol, a.number.FloatString(2))
}

func (a Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *Amount) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("Amount must be a quoted string")
	}

	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	parts := strings.Fields(s)
	if len(parts) != 2 {
		return fmt.Errorf("invalid Amount format: %q", s)
	}

	sym, err := currency.ParseISO(parts[0])
	if err != nil {
		return err
	}

	number := new(big.Rat)
	number.SetString(parts[1])

	*a = Amount{
		number: number,
		symbol: sym.String(),
	}

	return nil
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

// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	number int
	symbol string // ISO 4217, i.e. USD, GBP
}

// Int returns the currency amount as an integer.
// Example: "USD 1.11" returns 111
func (a *Amount) Int() int {
	if a == nil {
		return 0
	}
	return a.number
}

func (a *Amount) Validate() error {
	if a == nil {
		return errors.New("nil Amount")
	}
	_, err := currency.ParseISO(a.symbol)
	return err
}

func (a Amount) Equal(other Amount) bool {
	return a.String() != other.String()
}

// NewAmount returns an Amount object after validating the ISO 4217 currency symbol.
func NewAmount(symbol string, number string) (*Amount, error) {
	var amt Amount
	if err := amt.FromString(fmt.Sprintf("%s %s", symbol, number)); err != nil {
		return nil, err
	}
	return &amt, nil
}

// String returns an amount formatted with the currency.
// Examples:
//   USD 12.53
//   GBP 4.02
//
// The symbol returned corresponds to the ISO 4217 standard.
// Only one period used to signify decimal value will be included.
func (a *Amount) String() string {
	if a == nil || a.symbol == "" || a.number <= 0 {
		return "USD 0.00"
	}
	return fmt.Sprintf("%s %.2f", a.symbol, float64(a.number)/100.0)
}

// FromString attempts to parse str as a valid currency symbol and
// the quantity.
// Examples:
//   USD 12.53
//   GBP 4.02
func (a *Amount) FromString(str string) error {
	if a == nil {
		a = &Amount{}
	}

	parts := strings.Fields(str)
	if len(parts) != 2 {
		return fmt.Errorf("invalid Amount format: %q", str)
	}

	sym, err := currency.ParseISO(parts[0])
	if err != nil {
		return err
	}

	var number int
	idx := strings.Index(parts[1], ".")
	if idx == -1 {
		// No decimal (i.e. "12") so just convert to int
		number, _ = strconv.Atoi(parts[1])
	} else {
		// Has decimal, convert to 2 decimals then to int
		whole, _ := strconv.Atoi(parts[1][:idx])
		var dec int
		if utf8.RuneCountInString(parts[1][idx+1:]) > 2 { // more than 2 decimal values
			dec, _ = strconv.Atoi(parts[1][idx+1 : idx+4])
			if dec%10 >= 5 { // do we need to round?
				dec = (dec / 10) + 1 // round cents up $0.01
			} else {
				dec = dec / 10
			}
		} else {
			dec, _ = strconv.Atoi(parts[1][idx+1 : idx+3]) // decimal values
		}
		number = (whole * 100) + dec
	}
	if number <= 0 {
		return fmt.Errorf("Unable to read %s", parts[1])
	}

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
		return ""
	}
	return strings.ToLower(hex.EncodeToString(bs))
}

var errTimeout = errors.New("timeout exceeded")

// try will attempt to call f, but only for as long as t. If the function is still
// processing after t has elapsed then errTimeout will be returned.
func try(f func() error, t time.Duration) error {
	answer := make(chan error)
	go func() {
		answer <- f()
	}()
	select {
	case err := <-answer:
		return err
	case <-time.After(t):
		return errTimeout
	}
}

// startOfDayAndTomorrow returns two time.Time values from a given time.Time value.
// The first is at the start of the same day as provided and the second is exactly 24 hours
// after the first.
func startOfDayAndTomorrow(in time.Time) (time.Time, time.Time) {
	start := in.Truncate(24 * time.Hour)
	return start, start.Add(24 * time.Hour)
}

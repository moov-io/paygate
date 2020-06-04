// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/currency"
)

var (
	// ErrDifferentCurrencies is returned when an operation on an Amount instance is attempted with another Amount of a different currency (symbol).
	ErrDifferentCurrencies = errors.New("different currencies")
)

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
	return a.String() == other.String()
}

// Plus returns an Amount of adding both Amount instances together.
// Currency symbols must match for Plus to return without errors.
func (a Amount) Plus(other Amount) (Amount, error) {
	if a.symbol != other.symbol {
		return a, ErrDifferentCurrencies
	}
	return Amount{number: a.number + other.number, symbol: a.symbol}, nil
}

// NewAmountFromInt returns an Amount object after converting an integer amount (in cents)
// and validating the ISO 4217 currency symbol.
func NewAmountFromInt(symbol string, number int) (*Amount, error) {
	return NewAmount(symbol, formattedNumber(number))
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
	return fmt.Sprintf("%s %s", a.symbol, formattedNumber(a.number))
}

func formattedNumber(number int) string {
	if number <= 0 {
		return "0.00"
	}
	if number < 10 {
		return fmt.Sprintf("0.0%d", number)
	}
	if number < 100 {
		return fmt.Sprintf("0.%d", number)
	}
	str := fmt.Sprintf("%d", number)
	parts := []string{str[:len(str)-2], str[len(str)-2:]}
	return strings.Join(parts, ".")
}

// ParseAmount attempts to read a string as a valid currency symbol and number.
// Examples:
//   USD 12.53
func ParseAmount(in string) (*Amount, error) {
	amt, err := NewAmount("USD", "0.00")
	if err != nil {
		return nil, err
	}
	if err := amt.FromString(in); err != nil {
		return nil, err
	}
	return amt, nil
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
		number, err = strconv.Atoi(parts[1])
		if err != nil {
			return err
		}
	} else {
		// Has decimal, convert to 2 decimals then to int
		whole, err := strconv.Atoi(parts[1][:idx])
		if err != nil {
			return err
		}
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
	if number < 0 {
		return fmt.Errorf("unable to read %s", parts[1])
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

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func TestAmount(t *testing.T) {
	// happy path
	amt, err := NewAmount("USD", "12.00")
	if err != nil {
		t.Error(err)
	}
	if v := amt.String(); v != "USD 12.00" {
		t.Errorf("got %q", v)
	}

	amt, err = NewAmount("USD", "12")
	if err != nil {
		t.Error(err)
	}
	if v := amt.String(); v != "USD 0.12" {
		t.Errorf("got %q", v)
	}

	// invalid
	_, err = NewAmount("", ".0")
	if err == nil {
		t.Errorf("expected error")
	}

	// very large number
	amt, err = NewAmount("USD", "10000000000000000.20")
	if err != nil {
		t.Error(err)
	}
	if v := amt.String(); v != "USD 10000000000000000.20" {
		t.Errorf("got %q", v)
	}
}

func TestAmount__NewAmountFromInt(t *testing.T) {
	if amt, _ := NewAmountFromInt("USD", 1266); amt.String() != "USD 12.66" {
		t.Errorf("got %q", amt.String())
	}
	if amt, _ := NewAmountFromInt("USD", 4112); amt.String() != "USD 41.12" {
		t.Errorf("got %q", amt.String())
	}
	if amt, _ := NewAmountFromInt("USD", 1000000000000000020); amt.String() != "USD 10000000000000000.20" {
		t.Errorf("got %q", amt.String())
	}
}

func TestAmount__Int(t *testing.T) {
	amt, _ := NewAmount("USD", "12.53")
	if v := amt.Int(); v != 1253 {
		t.Error(v)
	}

	// check rouding with .Int()
	amt, _ = NewAmount("USD", "14.562")
	if v := amt.Int(); v != 1456 {
		t.Error(v)
	}
	amt, _ = NewAmount("USD", "14.568")
	if v := amt.Int(); v != 1457 {
		t.Error(v)
	}

	// small amounts
	amt, _ = NewAmount("USD", "0.03")
	if v := amt.Int(); v != 3 {
		t.Error(v)
	}
	amt, _ = NewAmount("USD", "0.030")
	if v := amt.Int(); v != 3 {
		t.Error(v)
	}
	amt, _ = NewAmount("USD", "0.003")
	if v := amt.Int(); v != 0 {
		t.Error(v)
	}

	// Handle cases which failed with math/big.Rat
	amt, _ = NewAmount("USD", fmt.Sprintf("%.3f", 853.0/100.0))
	if v := amt.Int(); v != 853 {
		t.Error(v)
	}
	amt, _ = NewAmount("USD", fmt.Sprintf("%.3f", 6907./50.0))
	if v := amt.Int(); v != 13814 {
		t.Error(v)
	}
}

func TestAmount__FromString(t *testing.T) {
	amt := Amount{}
	if err := amt.FromString("fail"); err == nil {
		t.Error("exected error")
	}
	if err := amt.FromString("USD 12.00"); err != nil {
		t.Error(err)
	}
	if err := amt.Validate(); err != nil {
		t.Error(err)
	}

	if err := amt.FromString("USD invalid"); err == nil {
		t.Error("expected error")
	}
	if err := amt.FromString("USD 1234"); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else {
		if amt.Int() != 1234 {
			t.Errorf("got %v", amt.String())
		}
	}
}

func TestAmount__json(t *testing.T) {
	// valid
	raw := []byte(`"USD 12.03"`)
	amt := Amount{}
	if err := json.Unmarshal(raw, &amt); err != nil {
		t.Error(err.Error())
	}
	if amt.symbol != "USD" {
		t.Errorf("got %s", amt.symbol)
	}
	if n := math.Abs(float64(1203 - amt.number)); n > 0.1 {
		t.Errorf("v=%d, n=%.2f", amt.number, n)
	}

	// valid, but no fractional amount
	bs, err := json.Marshal(Amount{1200.0 / 1.0, "USD"})
	if err != nil {
		t.Error(err)
	}
	if v := string(bs); v != `"USD 12.00"` {
		t.Errorf("got %q", v)
	}

	// round away extra precision, 3/1000 = 0.003 (rounds to 0.00)
	bs, err = json.Marshal(Amount{0, "USD"})
	if err != nil {
		t.Error(err)
	}
	if v := string(bs); v != `"USD 0.00"` {
		t.Errorf("got %q", v)
	}

	// invalid
	in := []byte(`"other thing"`)
	if err := json.Unmarshal(in, &amt); err == nil {
		t.Errorf("expected error")
	}
}

// TestAmount__issue202 represents unmarshaling Amount from various values
// See: https://github.com/moov-io/paygate/issues/202
func TestAmount__issue202(t *testing.T) {
	var amt Amount

	// note 1l9.33 -- the 'l' isn't a 1
	if err := json.Unmarshal([]byte(`"USD 1l9.33"`), &amt); err == nil {
		t.Fatal("expected error")
	} else {
		if v := err.Error(); v != `strconv.Atoi: parsing "1l9": invalid syntax` {
			t.Errorf("got %s", err)
		}
	}
}

func TestAmount__Equal(t *testing.T) {
	type state struct {
		amount Amount
		other  Amount
	}
	testCases := []struct {
		name     string
		state    state
		expected bool
	}{
		{
			"Two amounts are equal",
			state{
				amount: Amount{number: 10, symbol: "USD"},
				other:  Amount{number: 10, symbol: "USD"},
			},
			true,
		},
		{
			"The numbers are the same but the symbols don't match",
			state{
				amount: Amount{number: 10, symbol: "USD"},
				other:  Amount{number: 10, symbol: "CAD"},
			},
			false,
		},
		{
			"The symbols are the same but the numbers don't match",
			state{
				amount: Amount{number: 10, symbol: "USD"},
				other:  Amount{number: 5, symbol: "USD"},
			},
			false,
		},
		{
			"The base amount is empty",
			state{
				amount: Amount{},
				other:  Amount{number: 10, symbol: "USD"},
			},
			false,
		},
		{
			"The other amount is empty",
			state{
				amount: Amount{number: 10, symbol: "USD"},
				other:  Amount{},
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.state.amount.Equal(tc.state.other)
			if result != tc.expected {
				t.Errorf("Unexpected result; amount: %#v; other: %#v; expected result: %#v; actual result: %#v", tc.state.amount, tc.state.other, tc.expected, result)
			}
		})
	}
}

func TestAmount__plus(t *testing.T) {
	amt1, _ := NewAmount("USD", "0.11")
	amt2, _ := NewAmount("USD", "0.13")

	if a, err := amt1.Plus(*amt2); err != nil {
		t.Fatal(err)
	} else {
		if v := a.String(); v != "USD 0.24" {
			t.Fatalf("got %v", v)
		}
	}

	// invalid case
	amt1.symbol = "GBP"
	if _, err := amt1.Plus(*amt2); err == nil {
		t.Error("expected error")
	} else {
		if err != ErrDifferentCurrencies {
			t.Errorf("got %T %#v", err, err)
		}
	}
}

func TestAmount__validate(t *testing.T) {
	amt := &Amount{}
	if err := amt.Validate(); err == nil {
		t.Error("expected error")
	}
	amt = nil
	if err := amt.Validate(); err == nil {
		t.Error("expected error")
	}
}

func TestAmount__zero(t *testing.T) {
	if amt, err := NewAmount("USD", "0.00"); err != nil {
		t.Fatalf("amt=%v error=%v", amt, err)
	}
	if amt, err := NewAmountFromInt("USD", 0); err != nil {
		t.Fatalf("amt=%v error=%v", amt, err)
	}
}

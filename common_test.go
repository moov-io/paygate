// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestAccountType(t *testing.T) {
	var tpe AccountType
	if !tpe.empty() {
		t.Error("expected empty")
	}
}

func TestAccountType__json(t *testing.T) {
	at := Checking

	// marshal
	bs, err := json.Marshal(&at)
	if err != nil {
		t.Fatal(err.Error())
	}
	if s := string(bs); s != `"checking"` {
		t.Errorf("got %q", s)
	}

	// unmarshal
	raw := []byte(`"Savings"`) // test other case
	if err := json.Unmarshal(raw, &at); err != nil {
		t.Error(err.Error())
	}
	if at != Savings {
		t.Errorf("got %s", at)
	}

	// expect failures
	raw = []byte("bad")
	if err := json.Unmarshal(raw, &at); err == nil {
		t.Error("expected error")
	}
}

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
}

func TestAmount__NewAmountFromInt(t *testing.T) {
	if amt, _ := NewAmountFromInt("USD", 1266); amt.String() != "USD 12.66" {
		t.Errorf("got %q", amt.String())
	}
	if amt, _ := NewAmountFromInt("USD", 4112); amt.String() != "USD 41.12" {
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

func TestStartOfDayAndTomorrow(t *testing.T) {
	now := time.Now()
	min, max := startOfDayAndTomorrow(now)

	if !min.Before(now) {
		t.Errorf("min=%v now=%v", min, now)
	}
	if !max.After(now) {
		t.Errorf("max=%v now=%v", max, now)
	}

	if v := max.Sub(min); v != 24*time.Hour {
		t.Errorf("max - min = %v", v)
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"strings"
	"testing"

	"github.com/moov-io/paygate/pkg/model"
)

func TestFixedLimits__Soft(t *testing.T) {
	cfg := &FixedLimits{
		SoftLimit: "USD 104.23",
	}
	amt, _ := model.NewAmount("USD", "101.00")
	if over, err := cfg.OverSoftLimit(amt); over || err != nil {
		t.Errorf("expected amount to pass: %v", err)
	}

	// exceed limit
	amt, _ = model.NewAmount("USD", "120.01")
	if over, err := cfg.OverSoftLimit(amt); !over || err != nil {
		t.Errorf("expected error: %v", err)
	}
}

func TestFixedLimits__Hard(t *testing.T) {
	cfg := &FixedLimits{
		HardLimit: "USD 104.23",
	}
	amt, _ := model.NewAmount("USD", "101.00")
	if over, err := cfg.OverHardLimit(amt); over || err != nil {
		t.Errorf("expected amount to pass: %v", err)
	}

	// exceed limit
	amt, _ = model.NewAmount("USD", "120.01")
	if over, err := cfg.OverHardLimit(amt); !over || err != nil {
		t.Errorf("expected error: %v", err)
	}
}

func TestFixedLimitsErr(t *testing.T) {
	cfg := &FixedLimits{
		SoftLimit: "invalid",
	}
	if _, err := cfg.overLimit(cfg.SoftLimit, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestFixedLimits__Validate(t *testing.T) {
	cfg := &Transfers{
		Limits: Limits{
			Fixed: &FixedLimits{
				SoftLimit: "invalid",
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "soft limit:") {
			t.Errorf("unexpected error: %q", err)
		}
	}

	// verify hard limit fails too
	cfg.Limits.Fixed.SoftLimit = "USD 1.23"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "hard limit:") {
			t.Errorf("unexpected error: %q", err)
		}
	}

	// fully successful
	cfg.Limits.Fixed.HardLimit = "USD 100.00"
	if err := cfg.Validate(); err != nil {
		t.Error(err)
	}
}

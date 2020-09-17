// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"testing"

	"github.com/moov-io/paygate/pkg/client"
)

func TestFixedLimits__Soft(t *testing.T) {
	cfg := &FixedLimits{
		SoftLimit: 10423,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.OverSoftLimit(client.Amount{Value: 104}) {
		t.Error("expected under limit")
	}

	// invalid
	cfg.SoftLimit = -10
	if err := cfg.Validate(); err == nil {
		t.Error("expected error")
	}
}

func TestFixedLimits__Hard(t *testing.T) {
	cfg := &FixedLimits{
		SoftLimit: 100,
		HardLimit: 10423,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.OverHardLimit(client.Amount{Value: 104}) {
		t.Error("expected under limit")
	}

	// invalid
	cfg.HardLimit = -10
	if err := cfg.Validate(); err == nil {
		t.Error("expected error")
	}
}

func TestFixedLimits__Validate(t *testing.T) {
	cfg := &Transfers{
		Limits: Limits{
			Fixed: &FixedLimits{
				SoftLimit: -1,
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error")
	}

}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"fmt"

	"github.com/moov-io/paygate/pkg/client"
)

type Transfers struct {
	Limits Limits
}

func (cfg Transfers) Validate() error {
	if err := cfg.Limits.Validate(); err != nil {
		return fmt.Errorf("limits: %v", err)
	}
	return nil
}

type Limits struct {
	Fixed *FixedLimits
}

func (cfg Limits) Validate() error {
	if err := cfg.Fixed.Validate(); err != nil {
		return fmt.Errorf("fixed limits: %v", err)
	}
	return nil
}

type FixedLimits struct {
	// SoftLimit is a numerical value which is used to force created Transfer
	// objects into the REVIEWABLE status for manual approval prior to upload.
	SoftLimit int64

	// HardLimit is a numerical value. No Transfer amount is allowed to exceed this value
	// when specified.
	HardLimit int64
}

func (cfg *FixedLimits) Validate() error {
	if cfg == nil {
		return nil
	}
	if cfg.SoftLimit <= 0 || cfg.HardLimit < 0 {
		return fmt.Errorf("unexpected limits: SoftLimit=%d HardLimit=%d", cfg.SoftLimit, cfg.HardLimit)
	}
	return nil
}

func (cfg *FixedLimits) OverSoftLimit(amt client.Amount) bool {
	return cfg.overLimit(cfg.SoftLimit, amt)
}

func (cfg *FixedLimits) OverHardLimit(amt client.Amount) bool {
	return cfg.overLimit(cfg.HardLimit, amt)
}

func (cfg *FixedLimits) overLimit(limit int64, amt client.Amount) bool {
	return int64(amt.Value) > limit
}

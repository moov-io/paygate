// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"time"

	"github.com/moov-io/paygate/pkg/model"
)

type Transfers struct {
	Inbound Inbound
	Limits  Limits
}

func (cfg Transfers) Validate() error {
	if err := cfg.Limits.Validate(); err != nil {
		return fmt.Errorf("limits: %v", err)
	}
	return nil
}

type Inbound struct {
	Interval time.Duration // TODO(adam): moved from cfg.ODFI.Inbound
	Stream   *StreamPipeline
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
	SoftLimit string

	// HardLimit is a numerical value. No Transfer amount is allowed to exceed this value.
	HardLimit string
}

func (cfg *FixedLimits) Validate() error {
	if cfg == nil {
		return nil
	}
	if _, err := model.ParseAmount(cfg.SoftLimit); err != nil {
		return fmt.Errorf("soft limit: %v", err)
	}
	if _, err := model.ParseAmount(cfg.HardLimit); err != nil {
		return fmt.Errorf("hard limit: %v", err)
	}
	return nil
}

func (cfg *FixedLimits) OverSoftLimit(amt *model.Amount) (bool, error) {
	return cfg.overLimit(cfg.SoftLimit, amt)
}

func (cfg *FixedLimits) OverHardLimit(amt *model.Amount) (bool, error) {
	return cfg.overLimit(cfg.HardLimit, amt)
}

func (cfg *FixedLimits) overLimit(limit string, amt *model.Amount) (bool, error) {
	lmt, err := model.ParseAmount(limit)
	if err != nil {
		return true, err
	}
	return amt.Int() > lmt.Int(), nil
}

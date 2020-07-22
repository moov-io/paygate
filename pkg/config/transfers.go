// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"github.com/moov-io/paygate/pkg/model"
)

type Transfers struct {
	Limits Limits
}

type Limits struct {
	Fixed *FixedLimits
}

type FixedLimits struct {
	// SoftLimit is a numerical value which is used to force created Transfer
	// objects into the REVIEWABLE status for manual approval prior to upload.
	SoftLimit string

	// HardLimit is a numerical value. No Transfer amount is allowed to exceed this value.
	HardLimit string
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

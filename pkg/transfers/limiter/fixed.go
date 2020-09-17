// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package limiter

import (
	"fmt"

	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

type fixedLimiter struct {
	cfg *config.FixedLimits
}

func newFixedLimiter(cfg *config.FixedLimits) (Checker, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &fixedLimiter{cfg: cfg}, nil
}

func (l *fixedLimiter) Accept(namespace string, xfer *client.Transfer) error {
	if l.cfg.OverHardLimit(xfer.Amount) {
		return fmt.Errorf("fixedLimiter: %v", ErrOverLimits)
	}
	if l.cfg.OverSoftLimit(xfer.Amount) {
		return fmt.Errorf("fixedLimiter: %v", ErrReviewableTransfer)
	}
	return nil
}

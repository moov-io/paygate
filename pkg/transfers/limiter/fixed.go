// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package limiter

import (
	"fmt"

	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/model"
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

func (l *fixedLimiter) Accept(userID string, xfer *client.Transfer) error {
	amt, err := model.ParseAmount(xfer.Amount)
	if err != nil {
		return fmt.Errorf("fixedLimiter: unable to parse transfer amount: %v", err)
	}
	if over, err := l.cfg.OverHardLimit(amt); over || err != nil {
		if over {
			return fmt.Errorf("fixedLimiter: %v", ErrOverLimits)
		}
		if err != nil {
			return fmt.Errorf("fixedLimiter: hard limit parsing error: %v", err)
		}
	} else {
		// soft limit checks
		if over, err := l.cfg.OverSoftLimit(amt); over && err == nil {
			return fmt.Errorf("fixedLimiter: %v", ErrReviewableTransfer)
		} else {
			if err != nil {
				return fmt.Errorf("fixedLimiter: soft limit parsing error: %v", err)
			}
		}
	}
	return nil
}

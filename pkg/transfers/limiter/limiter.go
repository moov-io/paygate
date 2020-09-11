// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package limiter

import (
	"errors"

	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

var (
	ErrReviewableTransfer = errors.New("require manual review")
	ErrOverLimits         = errors.New("rejected transfer - over all limits")
)

type Checker interface {
	Accept(tenantID string, xfer *client.Transfer) error
}

func New(cfg config.Limits) (Checker, error) {
	if cfg.Fixed != nil {
		return newFixedLimiter(cfg.Fixed)
	}
	return &passingLimiter{}, nil
}

type passingLimiter struct{}

// Accept always returns no error for the passingLimiter
func (l *passingLimiter) Accept(tenantID string, xfer *client.Transfer) error {
	return nil
}

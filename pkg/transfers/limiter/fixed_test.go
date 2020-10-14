// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package limiter

import (
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

func TestFixedLimiter(t *testing.T) {
	limit, err := newFixedLimiter(&config.FixedLimits{
		SoftLimit: 111,
		HardLimit: 222,
	})
	if err != nil {
		t.Fatal(err)
	}

	organization := base.ID()
	xfer := &client.Transfer{
		Amount: client.Amount{
			Currency: "USD",
			Value:    100,
		},
	}
	// successful transfer
	if err := limit.Accept(organization, xfer); err != nil {
		t.Fatal(err)
	}

	// reviewable transfer
	xfer.Amount = client.Amount{
		Currency: "USD",
		Value:    133,
	}
	if err := limit.Accept(organization, xfer); err != nil {
		if !strings.Contains(err.Error(), ErrReviewableTransfer.Error()) {
			t.Fatalf("unexpected error: %q", err)
		}
	}

	// reject Transfer
	xfer.Amount = client.Amount{
		Currency: "USD",
		Value:    456,
	}
	if err := limit.Accept(organization, xfer); err != nil {
		if !strings.Contains(err.Error(), ErrOverLimits.Error()) {
			t.Fatalf("unexpected error: %q", err)
		}
	}
}

func TestFixedLimiterErr(t *testing.T) {
	if _, err := newFixedLimiter(&config.FixedLimits{}); err == nil {
		t.Error("expected error")
	}
}

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
		SoftLimit: "USD 1.11",
		HardLimit: "USD 2.22",
	})
	if err != nil {
		t.Fatal(err)
	}

	tenantID := base.ID()
	xfer := &client.Transfer{
		Amount: "USD 1.00",
	}
	// successful transfer
	if err := limit.Accept(tenantID, xfer); err != nil {
		t.Fatal(err)
	}

	// reviewable transfer
	xfer.Amount = "USD 1.33"
	if err := limit.Accept(tenantID, xfer); err != nil {
		if !strings.Contains(err.Error(), ErrReviewableTransfer.Error()) {
			t.Fatalf("unexpected error: %q", err)
		}
	}

	// reject Transfer
	xfer.Amount = "USD 4.56"
	if err := limit.Accept(tenantID, xfer); err != nil {
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

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"os"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/config"
)

func testPagerDutyClient(t *testing.T) *PagerDuty {
	t.Helper()

	cfg := &config.PagerDuty{
		ApiKey:     os.Getenv("PAGERDUTY_API_KEY"),
		From:       "adam@moov.io",
		ServiceKey: "PM8YUZY", // paygate
	}
	if cfg.ApiKey == "" {
		t.Skip("missing PagerDuty api key")
	}

	client, err := NewPagerDuty(cfg)
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func TestPagerDuty(t *testing.T) {
	pd := testPagerDutyClient(t)

	if err := pd.Ping(); err != nil {
		t.Fatal(err)
	}

	file := ach.NewFile()

	if err := pd.Info(&Message{
		Direction: Download,
		Filename:  "20200529-140002-1.ach",
		File:      file,
	}); err != nil {
		t.Fatal(err)
	}

	if err := pd.Critical(&Message{
		Direction: Upload,
		Filename:  "20200529-140002-2.ach",
		File:      file,
	}); err != nil {
		t.Fatal(err)
	}
}

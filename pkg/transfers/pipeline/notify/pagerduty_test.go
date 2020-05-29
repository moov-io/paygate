// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"testing"

	"github.com/moov-io/paygate/pkg/config"
)

func TestPagerDuty(t *testing.T) {
	pd, err := NewPagerDuty(&config.PagerDuty{})
	if err != nil {
		t.Fatal(err)
	}

	msg := &Message{
		Direction: Download,
		Filename:  "20200529-140002.ach",
	}

	if err := pd.Info(msg); err != nil {
		t.Fatal(err)
	}

	if err := pd.Critical(msg); err != nil {
		t.Fatal(err)
	}
}

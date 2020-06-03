// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"testing"

	"github.com/moov-io/paygate/pkg/config"
)

func TestSlack(t *testing.T) {
	slack, err := NewSlack(&config.Slack{})
	if err != nil {
		t.Fatal(err)
	}

	msg := &Message{
		Direction: Download,
		Filename:  "20200529-152259.ach",
	}

	if err := slack.Info(msg); err != nil {
		t.Fatal(err)
	}

	if err := slack.Critical(msg); err != nil {
		t.Fatal(err)
	}
}

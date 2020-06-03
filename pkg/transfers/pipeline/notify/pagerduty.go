// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"fmt"

	"github.com/moov-io/paygate/pkg/config"
)

type PagerDuty struct {
	// client pagerduty.Client
}

func NewPagerDuty(cfg *config.PagerDuty) (*PagerDuty, error) {
	return nil, nil
}

func (pd *PagerDuty) Info(msg *Message) error {
	body := "successful " + pagerdutyMessage(msg)
	fmt.Printf("[INFO] pagerduty: body=%q\n", body)
	return nil
}

func (pd *PagerDuty) Critical(msg *Message) error {
	body := "failed to " + pagerdutyMessage(msg)
	fmt.Printf("[CRITICAL] pagerduty: body=%q\n", body)
	return nil
}

func pagerdutyMessage(msg *Message) string {
	return fmt.Sprintf("%s of %s", msg.Direction, msg.Filename)
}

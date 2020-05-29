// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"fmt"

	"github.com/moov-io/paygate/pkg/config"
)

type Slack struct {
	// client slack.Client
}

func NewSlack(cfg *config.Slack) (*Slack, error) {
	return nil, nil
}

func (s *Slack) Info(msg *Message) error {
	body := "successful " + slackMessage(msg)
	fmt.Printf("[INFO] slack: body=%q\n", body)
	return nil
}

func (s *Slack) Critical(msg *Message) error {
	body := "failed to " + slackMessage(msg)
	fmt.Printf("[CRITICAL] slack: body=%q\n", body)
	return nil
}

func slackMessage(msg *Message) string {
	return fmt.Sprintf("%s of %s", msg.Direction, msg.Filename)
}

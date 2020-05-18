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
	fmt.Printf("[INFO] slack: message=%#v", msg)
	return nil
}

func (s *Slack) Critical(msg *Message) error {
	fmt.Printf("[CRITICAL] slack: message=%#v", msg)
	return nil
}

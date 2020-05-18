// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"github.com/moov-io/paygate/pkg/config"
)

// MultiSender is a Sender which will attempt to send each Message to every
// included Sender and returns the first error encountered.
type MultiSender struct {
	senders []Sender
}

func NewMultiSender(cfg *config.PipelineNotifications) (*MultiSender, error) {
	ms := &MultiSender{}
	if cfg == nil {
		return ms, nil
	}
	if cfg.Slack != nil {
		sender, err := NewSlack(cfg.Slack)
		if err != nil {
			return nil, err
		}
		ms.senders = append(ms.senders, sender)
	}
	if cfg.PagerDuty != nil {
		sender, err := NewPagerDuty(cfg.PagerDuty)
		if err != nil {
			return nil, err
		}
		ms.senders = append(ms.senders, sender)
	}
	return ms, nil
}

func (ms *MultiSender) Info(msg *Message) error {
	var firstError error
	for i := range ms.senders {
		if err := ms.senders[i].Info(msg); err != nil && firstError == nil {
			firstError = err
		}
	}
	return firstError
}

func (ms *MultiSender) Critical(msg *Message) error {
	var firstError error
	for i := range ms.senders {
		if err := ms.senders[i].Critical(msg); err != nil && firstError == nil {
			firstError = err
		}
	}
	return firstError
}

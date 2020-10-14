// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moov-io/paygate"
	"github.com/moov-io/paygate/pkg/config"
)

type Slack struct {
	client     *http.Client
	webhookURL string
}

func NewSlack(cfg *config.Slack) (*Slack, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Slack{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		webhookURL: strings.TrimSpace(cfg.WebhookURL),
	}, nil
}

type uploadStatus string

const (
	success = uploadStatus("successful")
	failed  = uploadStatus("failed")
)

func (s *Slack) Info(msg *Message) error {
	slackMsg := marshalSlackMessage(success, msg)
	return s.send(slackMsg)
}

func (s *Slack) Critical(msg *Message) error {
	slackMsg := marshalSlackMessage(failed, msg)
	return s.send(slackMsg)
}

func marshalSlackMessage(status uploadStatus, msg *Message) string {
	slackMsg := fmt.Sprintf("%s %s of %s", status, msg.Direction, msg.Filename)
	if msg.Hostname != "" {
		if msg.Direction == Upload {
			slackMsg += fmt.Sprintf(" to %s", msg.Hostname)
		} else {
			slackMsg += fmt.Sprintf(" from %s", msg.Hostname)
		}
	}
	slackMsg += " with ODFI server"

	return slackMsg
}

type webhook struct {
	Text string `json:"text"`
}

func (s *Slack) send(msg string) error {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(&webhook{
		Text: msg,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", s.webhookURL, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("moov/paygate %v slack notifier", paygate.Version))

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

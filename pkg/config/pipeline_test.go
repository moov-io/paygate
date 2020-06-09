// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"testing"
)

func TestPipeline(t *testing.T) {
	cfg := Pipeline{}
	if err := cfg.Validate(); err != nil {
		t.Error(err)
	}
}

func TestPreupload(t *testing.T) {
	cfg := &PreUpload{
		GPG: &GPG{
			KeyFile: "", // intentionally left blank
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error")
	}
}

func TestStreamPipeline(t *testing.T) {
	cfg := &StreamPipeline{
		InMem: &InMemPipeline{
			URL: "", // intentionally left blank
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error(err)
	}

	cfg.InMem = nil
	cfg.Kafka = &KafkaPipeline{
		Brokers: []string{},
	}
	if err := cfg.Validate(); err == nil {
		t.Error(err)
	}
}

func TestPipelineNotifications(t *testing.T) {
	cfg := &PipelineNotifications{
		Email: &Email{
			From: "", // intentionally left blank
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error(err)
	}
	cfg.Email = nil

	cfg.PagerDuty = &PagerDuty{ApiKey: ""}
	if err := cfg.Validate(); err == nil {
		t.Error(err)
	}
	cfg.PagerDuty = nil

	cfg.Slack = &Slack{WebhookURL: ""}
	if err := cfg.Validate(); err == nil {
		t.Error(err)
	}
}

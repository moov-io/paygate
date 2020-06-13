// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"errors"
	"fmt"
	"text/template"
)

var (
	DefaultEmailTemplate = template.Must(template.New("email").Parse(`
A file has been {{ .Verb }}ed from {{ .CompanyName }}: {{ .Filename }}
Debits:  ${{ .DebitTotal | printf "%.2f" }}
Credits: ${{ .CreditTotal | printf "%.2f" }}

Batches: {{ .BatchCount }}
Total Entries: {{ .EntryCount }}
`))
)

type Pipeline struct {
	PreUpload     *PreUpload             `yaml:"pre_upload" json:"pre_upload"`
	Output        *Output                `yaml:"output" json:"output"`
	Merging       *Merging               `yaml:"merging" json:"merging"`
	AuditTrail    *AuditTrail            `yaml:"audit_trail" json:"audit_trail"`
	Stream        *StreamPipeline        `yaml:"stream" json:"stream"`
	Notifications *PipelineNotifications `yaml:"notifications" json:"notifications"`
}

func (cfg Pipeline) Validate() error {
	if err := cfg.PreUpload.Validate(); err != nil {
		return fmt.Errorf("pre-upload: %v", err)
	}
	if err := cfg.Output.Validate(); err != nil {
		return fmt.Errorf("output: %v", err)
	}
	if err := cfg.Merging.Validate(); err != nil {
		return fmt.Errorf("merging: %v", err)
	}
	if err := cfg.AuditTrail.Validate(); err != nil {
		return fmt.Errorf("audit-trail: %v", err)
	}
	if err := cfg.Stream.Validate(); err != nil {
		return fmt.Errorf("stream: %v", err)
	}
	if err := cfg.Notifications.Validate(); err != nil {
		return fmt.Errorf("notifications: %v", err)
	}
	return nil
}

type PreUpload struct {
	GPG *GPG `yaml:"gpg" json:"gpg"`
}

func (cfg *PreUpload) Validate() error {
	if cfg == nil {
		return nil
	}
	if cfg.GPG != nil && cfg.GPG.KeyFile == "" {
		return errors.New("gpg: missing key file")
	}
	return nil
}

type GPG struct {
	KeyFile string `yaml:"key_file" json:"key_file"`
}

type Output struct {
	Format string `yaml:"format" json:"format"`
}

func (cfg *Output) Validate() error {
	return nil
}

type Merging struct {
	Directory string `yaml:"directory" json:"directory"`
}

func (cfg *Merging) Validate() error {
	return nil
}

type AuditTrail struct {
	BucketURI string `yaml:"bucket_uri" json:"bucket_uri"`
	GPG       *GPG   `yaml:"gpg" json:"gpg"`
}

func (cfg *AuditTrail) Validate() error {
	if cfg == nil {
		return nil
	}
	if cfg.BucketURI == "" {
		return errors.New("missing bucket_uri")
	}
	return nil
}

type StreamPipeline struct {
	InMem *InMemPipeline `yaml:"inmem" json:"inmem"`
	Kafka *KafkaPipeline `yaml:"kafka" json:"kafka"`
}

func (cfg *StreamPipeline) Validate() error {
	if cfg == nil {
		return nil
	}
	if cfg.InMem != nil && cfg.InMem.URL == "" {
		return errors.New("inmem: missing stream url")
	}
	if k := cfg.Kafka; k != nil {
		if len(k.Brokers) == 0 || k.Group == "" || k.Topic == "" {
			return errors.New("kafka: missing brokers, group, or topic")
		}
	}
	return nil
}

type InMemPipeline struct {
	URL string `yaml:"url" json:"url"`
}

type KafkaPipeline struct {
	Brokers []string `yaml:"brokers" json:"brokers"`
	Group   string   `yaml:"group" json:"group"`
	Topic   string   `yaml:"topic" json:"topic"`
}

type PipelineNotifications struct {
	Email     *Email     `yaml:"email" json:"email"`
	PagerDuty *PagerDuty `yaml:"pagerduty" json:"pagerduty"`
	Slack     *Slack     `yaml:"slack" json:"slack"`
}

func (cfg *PipelineNotifications) Validate() error {
	if cfg == nil {
		return nil
	}
	if e := cfg.Email; e != nil {
		if e.From == "" || len(e.To) == 0 || e.ConnectionURI == "" || e.CompanyName == "" {
			return errors.New("email: missing configs")
		}
	}
	if cfg.PagerDuty != nil && cfg.PagerDuty.ApiKey == "" {
		return errors.New("pagerduty: missing api key")
	}
	if err := cfg.Slack.Validate(); err != nil {
		return err
	}
	return nil
}

type Email struct {
	From string   `yaml:"from" json:"from"`
	To   []string `yaml:"to" json:"to"`

	// ConnectionURI is a URI used to connect with a remote SFTP server.
	// This config typically needs to contain enough values to successfully
	// authenticate with the server.
	// - insecure_skip_verify is an optional parameter for disabling certificate verification
	//
	// Example: smtps://user:pass@localhost:1025/?insecure_skip_verify=true
	ConnectionURI string `yaml:"connection_uri" json:"connection_uri"`

	Template    string `yaml:"template" json:"template"`
	CompanyName string `yaml:"company_name" json:"company_name"` // e.g. Moov
}

func (e *Email) Tmpl() *template.Template {
	if e == nil || e.Template == "" {
		return DefaultEmailTemplate
	}
	return template.Must(template.New("custom-email").Parse(e.Template))
}

type PagerDuty struct {
	ApiKey string `yaml:"api_key" json:"api_key"`
}

type Slack struct {
	WebhookURL string `yaml:"webhook_url" json:"webhook_url"`
}

func (cfg *Slack) Validate() error {
	if cfg == nil {
		return nil
	}
	if cfg.WebhookURL == "" {
		return errors.New("slack: missing webhook url")
	}
	return nil
}

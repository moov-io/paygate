// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
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
	Merging       *Merging               `yaml:"merging"`
	Stream        *StreamPipeline        `yaml:"stream"`
	Notifications *PipelineNotifications `yaml:"notifications"`
}

type Merging struct {
	Directory string `yaml:"directory"`
}

type StreamPipeline struct {
	InMem *InMemPipeline `yaml:"inmem"`
	Kafka *KafkaPipeline `yaml:"kafka"`
}

type InMemPipeline struct {
	URL string `yaml:"url"`
}

type KafkaPipeline struct {
	Brokers []string `yaml:"brokers"`
	Group   string   `yaml:"group"`
	Topic   string   `yaml:"topic"`
}

type PipelineNotifications struct {
	Email     *Email     `yaml:"email"`
	PagerDuty *PagerDuty `yaml:"pagerduty"`
	Slack     *Slack     `yaml:"slack"`
}

type Email struct {
	From string   `yaml:"from"`
	To   []string `yaml:"to"`

	// ConnectionURI is a URI used to connect with a remote SFTP server.
	// This config typically needs to contain enough values to successfully
	// authenticate with the server.
	// - insecure_skip_verify is an optional parameter for disabling certificate verification
	//
	// Example: smtps://user:pass@localhost:1025/?insecure_skip_verify=true
	ConnectionURI string `yaml:"connection_uri"`

	Template    string `yaml:"template"`
	CompanyName string `yaml:"company_name"` // e.g. Moov
}

func (e *Email) Tmpl() *template.Template {
	if e == nil || e.Template == "" {
		return DefaultEmailTemplate
	}
	return template.Must(template.New("custom-email").Parse(e.Template))
}

type PagerDuty struct {
	ApiKey string `yaml:"api_key"`
}

type Slack struct {
	ApiKey string `yaml:"api_key"`
}

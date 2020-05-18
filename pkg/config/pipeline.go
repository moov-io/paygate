// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

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
	Slack     *Slack     `yaml:"slack"`
	PagerDuty *PagerDuty `yaml:"pagerduty"`
}

type Slack struct {
	ApiKey string `yaml:"api_key"`
}

type PagerDuty struct {
	ApiKey string `yaml:"api_key"`
}

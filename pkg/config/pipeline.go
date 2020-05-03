// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"time"
)

type Pipeline struct {
	Filesystem *FilesystemPipeline `yaml:"filesystem"`
	Stream     *StreamPipeline     `yaml:"stream"`
}

type FilesystemPipeline struct {
	Interval  time.Duration `yaml:"interval"`
	Directory string        `yaml:"directory"`
}

type StreamPipeline struct {
	Kafka *KafkaPipeline `yaml:"kafka"`
}

type KafkaPipeline struct {
	Brokers []string `yaml:"brokers"`
	Group   string   `yaml:"group"`
	Topic   string   `yaml"topic"`
}

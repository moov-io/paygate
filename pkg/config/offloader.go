// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"time"
)

type Offloader struct {
	Local  *LocalOffloader  `yaml:"local"`
	Stream *StreamOffloader `yaml:"stream"`
}

type LocalOffloader struct {
	Interval  time.Duration `yaml:"interval"`
	Directory string        `yaml:"directory"`
}

type StreamOffloader struct {
	// aka kafka configs
}

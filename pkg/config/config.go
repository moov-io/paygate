// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Logger    log.Logger
	LogFormat string `yaml:"log_format"`

	Http  HTTP  `yaml:"http"`
	Admin Admin `yaml:"admin"`

	ODFI ODFI `yaml:"odfi"`
}

func Empty() *Config {
	return &Config{
		Logger: log.NewNopLogger(),
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := Empty()

	if path != "" {
		bs, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("config: read %s: %v", path, err)
		}
		if err := yaml.Unmarshal(bs, cfg); err != nil {
			return nil, fmt.Errorf("config: unmarshal %s: %v", path, err)
		}
	}

	// Setup our Logger
	if strings.EqualFold(cfg.LogFormat, "json") {
		cfg.Logger = log.NewJSONLogger(os.Stderr)
	} else {
		cfg.Logger = log.NewLogfmtLogger(os.Stderr)
	}
	cfg.Logger = log.With(cfg.Logger, "ts", log.DefaultTimestampUTC)
	cfg.Logger = log.With(cfg.Logger, "caller", log.DefaultCaller)

	return cfg, nil
}

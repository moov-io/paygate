// Copyright 2019 The Moov Authors
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
}

func Empty() *Config {
	cfg := Config{
		Logger: log.NewNopLogger(),
	}
	return &cfg
}

func LoadConfig(path string, logFormat *string) (*Config, error) {
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

	err := OverrideWithEnvVars(cfg)
	if err != nil {
		return nil, err
	}

	// Setup our Logger
	if *logFormat != "" {
		cfg.LogFormat = *logFormat
	}
	if strings.EqualFold(cfg.LogFormat, "json") {
		cfg.Logger = log.NewJSONLogger(os.Stderr)
	} else {
		cfg.Logger = log.NewLogfmtLogger(os.Stderr)
	}
	cfg.Logger = log.With(cfg.Logger, "ts", log.DefaultTimestampUTC)
	cfg.Logger = log.With(cfg.Logger, "caller", log.DefaultCaller)

	return cfg, nil
}

func override(env string, field *string) {
	if v := os.Getenv(env); v != "" {
		*field = v
	}
}

func OverrideWithEnvVars(cfg *Config) error {
	var err error

	return err
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moov-io/paygate/internal/util"

	"github.com/go-kit/kit/log"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Logger    log.Logger
	LogFormat string `yaml:"log_format"`

	Customers *CustomersConfig `yaml:"customers"`
}

type CustomersConfig struct {
	Disabled bool   `yaml:"disabled"`
	Endpoint string `yaml:"endpoint"`

	OFACBatchSize    int           `yaml:"ofacBatchSize"`
	OFACRefreshEvery time.Duration `yaml:"ofacRefreshEvery"`
}

func Empty() *Config {
	cfg := Config{
		Logger:    log.NewNopLogger(),
		Customers: &CustomersConfig{},
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

	override("CUSTOMERS_ENDPOINT", &cfg.Customers.Endpoint)
	if v := os.Getenv("CUSTOMERS_CALLS_DISABLED"); v != "" {
		cfg.Customers.Disabled = util.Yes(v)
	}
	if v := os.Getenv("CUSTOMERS_OFAC_BATCH_SIZE"); v != "" {
		cfg.Customers.OFACBatchSize, err = strconv.Atoi(v)
	}
	if cfg.Customers.OFACBatchSize == 0 {
		cfg.Customers.OFACBatchSize = 100
	}
	if v := os.Getenv("CUSTOMERS_OFAC_REFRESH_EVERY"); v != "" {
		cfg.Customers.OFACRefreshEvery, err = time.ParseDuration(v)
	}
	if cfg.Customers.OFACRefreshEvery == 0*time.Second {
		cfg.Customers.OFACRefreshEvery = 7 * 24 * time.Hour // weekly
	}

	return err
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/moov-io/base/http/bind"

	"github.com/go-kit/kit/log"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Logger  log.Logger `yaml:"-" json:"-"`
	Logging Logging    `yaml:"logging" json:"logging"`

	Http  HTTP  `yaml:"http" json:"http"`
	Admin Admin `yaml:"admin" json:"admin"`

	Database Database `yaml:"database" json:"database"`

	ODFI       ODFI       `yaml:"odfi" json:"odfi"`
	Pipeline   Pipeline   `yaml:"pipeline" json:"pipeline"`
	Validation Validation `yaml:"validation" json:"validation"`

	Customers Customers `yaml:"customers" json:"customers"`
}

type Logging struct {
	Format string `yaml:"format" json:"format"`
	Level  string `yaml:"level" json:"level"`
}

func Empty() *Config {
	return &Config{
		Logger: log.NewNopLogger(),
		Admin: Admin{
			BindAddress: bind.Admin("paygate"),
		},
		Http: HTTP{
			BindAddress: bind.HTTP("paygate"),
		},
		Database: Database{
			// Set the default path inside this path if no other database is defined.
			SQLite: &SQLite{
				Path: "paygate.db",
			},
		},
	}
}

func FromFile(path string) (*Config, error) {
	cfg := Empty()
	if path != "" {
		bs, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("config: read %s: %v", path, err)
		}
		return Read(bs)
	}
	cfg = setupLogger(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Read(data []byte) (*Config, error) {
	cfg := Empty()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal: %v", err)
	}
	cfg = setupLogger(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func setupLogger(cfg *Config) *Config {
	if strings.EqualFold(cfg.Logging.Format, "json") {
		cfg.Logger = log.NewJSONLogger(os.Stderr)
	} else {
		cfg.Logger = log.NewLogfmtLogger(os.Stderr)
	}

	cfg.Logger = log.With(cfg.Logger, "ts", log.DefaultTimestampUTC)
	cfg.Logger = log.With(cfg.Logger, "caller", log.DefaultCaller)

	return cfg
}

// Validate checks a Config fields and performs various confirmations
// their values conform to expectations.
func (cfg *Config) Validate() error {
	if cfg == nil {
		return errors.New("missing Config")
	}

	if err := cfg.ODFI.Validate(); err != nil {
		return fmt.Errorf("odfi: %v", err)
	}
	if err := cfg.Pipeline.Validate(); err != nil {
		return fmt.Errorf("pipeline: %v", err)
	}
	if err := cfg.Validation.Validate(); err != nil {
		return fmt.Errorf("validation: %v", err)
	}

	if err := cfg.Customers.Validate(); err != nil {
		return fmt.Errorf("customers: %v", err)
	}

	return nil
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/moov-io/base/http/bind"

	"github.com/go-kit/kit/log"
	"github.com/spf13/viper"
)

type Config struct {
	Logger  log.Logger `yaml:"-" json:"-"`
	Logging Logging

	Http  HTTP
	Admin Admin

	Database Database

	ODFI       ODFI
	Pipeline   Pipeline
	Transfers  Transfers
	Validation Validation

	Customers Customers
}

type Logging struct {
	Format string
	Level  string
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
	vip := viper.New()
	vip.SetConfigType("yaml")
	if err := vip.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("problem reading config: %v", err)
	}

	cfg := Empty()
	if err := vip.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("problem unmarshaling config: %v", err)
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
	if err := cfg.Transfers.Validate(); err != nil {
		return fmt.Errorf("transfers: %v", err)
	}
	if err := cfg.Validation.Validate(); err != nil {
		return fmt.Errorf("validation: %v", err)
	}

	if err := cfg.Customers.Validate(); err != nil {
		return fmt.Errorf("customers: %v", err)
	}

	return nil
}

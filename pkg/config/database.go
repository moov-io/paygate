// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"os"

	"github.com/moov-io/paygate/pkg/util"
)

type Database struct {
	SQLite *SQLite `yaml:"sqlite" json:"sqlite"`
	MySQL  *MySQL  `yaml:"mysql" json:"mysql"`
}

type SQLite struct {
	Path string `yaml:"path" json:"path"`
}

type MySQL struct {
	Address  string `yaml:"address" json:"address"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	Database string `yaml:"database" json:"database"`
}

func (cfg *MySQL) GetPassword() string {
	pass := os.Getenv("MYSQL_PASSWORD")
	if cfg == nil {
		return pass
	}
	return util.Or(pass, cfg.Password)
}

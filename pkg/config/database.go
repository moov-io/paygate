// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"os"

	"github.com/moov-io/paygate/pkg/util"
)

type Database struct {
	SQLite *SQLite
	MySQL  *MySQL
}

type SQLite struct {
	Path string
}

type MySQL struct {
	Address  string
	Username string
	Password string
	Database string
}

func (cfg *MySQL) GetPassword() string {
	pass := os.Getenv("MYSQL_PASSWORD")
	if cfg == nil {
		return pass
	}
	return util.Or(pass, cfg.Password)
}

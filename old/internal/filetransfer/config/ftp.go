// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"bytes"
	"fmt"
)

type FTPConfig struct {
	RoutingNumber string `yaml:"routingNumber"`
	Hostname      string `yaml:"hostname"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
}

func (cfg *FTPConfig) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("FTPConfig{RoutingNumber=%s, ", cfg.RoutingNumber))
	buf.WriteString(fmt.Sprintf("Hostname=%s, ", cfg.Hostname))
	buf.WriteString(fmt.Sprintf("Username=%s, ", cfg.Username))
	buf.WriteString(fmt.Sprintf("Password=%s}", maskPassword(cfg.Password)))
	return buf.String()
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"strings"
)

type Config struct {
	RoutingNumber string `json:"routingNumber" yaml:"routingNumber"`

	InboundPath  string `json:"inboundPath" yaml:"inboundPath"`
	OutboundPath string `json:"outboundPath" yaml:"outboundPath"`
	ReturnPath   string `json:"returnPath" yaml:"returnPath"`

	OutboundFilenameTemplate string `json:"outboundFilenameTemplate" yaml:"outboundFilenameTemplate"`

	AllowedIPs string

	FTP  *FTPConfig  `json:"ftp" yaml:"ftp"`
	SFTP *SFTPConfig `json:"sftp" yaml:"sftp"`
}

func (cfg *Config) FilenameTemplate() string {
	if cfg == nil || cfg.OutboundFilenameTemplate == "" {
		return DefaultFilenameTemplate
	}
	return cfg.OutboundFilenameTemplate
}

func (cfg *Config) splitAllowedIPs() []string {
	if cfg.AllowedIPs != "" {
		return strings.Split(cfg.AllowedIPs, ",")
	}
	return nil
}

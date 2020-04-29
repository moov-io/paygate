// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/moov-io/paygate/x/mask"
)

var (
	// DefaultFilenameTemplate is paygate's standard filename format for ACH files which are uploaded to an ODFI
	//
	// The format consists of a few parts: "year month day" timestamp, routing number, and sequence number
	//
	// Examples:
	//  - 20191010-987654320-1.ach
	//  - 20191010-987654320-1.ach.gpg (GPG encrypted)
	DefaultFilenameTemplate = `{{ date "20060102" }}-{{ .RoutingNumber }}-{{ .N }}.ach{{ if .GPG }}.gpg{{ end }}`
)

type ODFI struct {
	RoutingNumber string  `yaml:"routing_number"`
	Gateway       Gateway `yaml:"gateway"`

	InboundPath  string `yaml:"inbound_path"`
	OutboundPath string `yaml:"outbound_path"`
	ReturnPath   string `yaml:"return_path"`

	AllowedIPs               string `yaml:"allowed_ips"`
	OutboundFilenameTemplate string `yaml:"outbound_filename_template"`

	FTP  *FTP  `yaml:"ftp"`
	SFTP *SFTP `yaml:"sftp"`
}

func (cfg *ODFI) FilenameTemplate() string {
	if cfg == nil || cfg.OutboundFilenameTemplate == "" {
		return DefaultFilenameTemplate
	}
	return cfg.OutboundFilenameTemplate
}

func (cfg *ODFI) SplitAllowedIPs() []string {
	if cfg.AllowedIPs != "" {
		return strings.Split(cfg.AllowedIPs, ",")
	}
	return nil
}

type Gateway struct {
	Origin          string `yaml:"origin"`
	OriginName      string `yaml:"origin_name"`
	Destination     string `yaml:"destination"`
	DestinationName string `yaml:"destination_name"`
}

type FTP struct {
	Hostname string `yaml:"hostname"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func (cfg *FTP) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("FTP{Hostname=%s, ", cfg.Hostname))
	buf.WriteString(fmt.Sprintf("Username=%s, ", cfg.Username))
	buf.WriteString(fmt.Sprintf("Password=%s}", mask.Password(cfg.Password)))
	return buf.String()
}

type SFTP struct {
	Hostname string `yaml:"hostname"`
	Username string `yaml:"username"`

	Password         string `yaml:"password"`
	ClientPrivateKey string `yaml:"clientPrivateKey"`

	HostPublicKey string `yaml:"hostPublicKey"`
}

func (cfg *SFTP) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("SFTP{Hostname=%s, ", cfg.Hostname))
	buf.WriteString(fmt.Sprintf("Username=%s, ", cfg.Username))
	buf.WriteString(fmt.Sprintf("Password=%s, ", mask.Password(cfg.Password)))
	buf.WriteString(fmt.Sprintf("ClientPrivateKey:%v, ", cfg.ClientPrivateKey != ""))
	buf.WriteString(fmt.Sprintf("HostPublicKey:%v}, ", cfg.HostPublicKey != ""))
	return buf.String()
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moov-io/ach"
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

// ODFI holds all the configuration for sending and retrieving ACH files with
// a financial institution to originate files.
type ODFI struct {
	// RoutingNumber is a valid ABA routing number
	RoutingNumber string `yaml:"routing_number" json:"routing_number"`

	// Gateway holds FileHeader information which the ODFI requires is set
	// on all files uploaded.
	Gateway Gateway `yaml:"gateway" json:"gateway"`

	Cutoffs Cutoffs `yaml:"cutoffs" json:"cutoffs"`

	InboundPath  string `yaml:"inbound_path" json:"inbound_path"`
	OutboundPath string `yaml:"outbound_path" json:"outbound_path"`
	ReturnPath   string `yaml:"return_path" json:"return_path"`

	// AllowedIPs is a comma separated list of IP addresses and CIDR ranges
	// where connections are allowed. If this value is non-empty remote servers
	// not within these ranges will not be connected to.
	AllowedIPs string `yaml:"allowed_ips" json:"allowed_ips"`

	OutboundFilenameTemplate string `yaml:"outbound_filename_template" json:"outbound_filename_template"`

	FTP  *FTP  `yaml:"ftp" json:"ftp"`
	SFTP *SFTP `yaml:"sftp" json:"sftp"`

	Inbound Inbound `yaml:"inbound" json:"inbound"`

	Transfers Transfers `yaml:"transfers" json:"transfers"`

	Storage *Storage `yaml:"storage" json:"storage"`
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

func (cfg *ODFI) Validate() error {
	if cfg == nil {
		return errors.New("missing ODFI config")
	}
	if err := ach.CheckRoutingNumber(cfg.RoutingNumber); err != nil {
		return err
	}
	if err := cfg.Cutoffs.Validate(); err != nil {
		return err
	}
	return nil
}

type Gateway struct {
	Origin          string `yaml:"origin" json:"origin"`
	OriginName      string `yaml:"origin_name" json:"origin_name"`
	Destination     string `yaml:"destination" json:"destination"`
	DestinationName string `yaml:"destination_name" json:"destination_name"`
}

type Cutoffs struct {
	Timezone string   `yaml:"timezone" json:"timezone"`
	Windows  []string `yaml:"windows" json:"windows"`
}

func (cfg Cutoffs) Location() *time.Location {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil
	}
	return loc
}

func (cfg Cutoffs) Validate() error {
	if loc := cfg.Location(); loc == nil {
		return fmt.Errorf("unknown Timezone=%q", cfg.Timezone)
	}
	if len(cfg.Windows) == 0 {
		return errors.New("no cutoff windows")
	}
	return nil
}

type FTP struct {
	Hostname string `yaml:"hostname" json:"hostname"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`

	CAFilepath   string        `yaml:"ca_file" json:"ca_file"`
	DialTimeout  time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	DisabledEPSV bool          `yaml:"disabled_epsv" json:"disabled_epsv"`
}

func (cfg *FTP) CAFile() string {
	if cfg == nil {
		return ""
	}
	return cfg.CAFilepath
}

func (cfg *FTP) Timeout() time.Duration {
	if cfg == nil || cfg.DialTimeout == 0*time.Second {
		return 10 * time.Second
	}
	return cfg.DialTimeout
}

func (cfg *FTP) DisableEPSV() bool {
	if cfg == nil {
		return false
	}
	return cfg.DisabledEPSV
}

func (cfg *FTP) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("FTP{Hostname=%s, ", cfg.Hostname))
	buf.WriteString(fmt.Sprintf("Username=%s, ", cfg.Username))
	buf.WriteString(fmt.Sprintf("Password=%s}", mask.Password(cfg.Password)))
	return buf.String()
}

type SFTP struct {
	Hostname string `yaml:"hostname" json:"hostname"`
	Username string `yaml:"username" json:"username"`

	Password         string `yaml:"password" json:"password"`
	ClientPrivateKey string `yaml:"client_private_key" json:"client_private_key"`
	HostPublicKey    string `yaml:"host_public_key" json:"host_public_key"`

	DialTimeout           time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	MaxConnectionsPerFile int           `yaml:"max_connections_per_file" json:"max_connections_per_file"`
	MaxPacketSize         int           `yaml:"max_packet_size" json:"max_packet_size"`
}

func (cfg *SFTP) Timeout() time.Duration {
	if cfg == nil || cfg.DialTimeout == 0*time.Second {
		return 10 * time.Second
	}
	return cfg.DialTimeout
}

func (cfg *SFTP) MaxConnections() int {
	if cfg == nil || cfg.MaxConnectionsPerFile == 0 {
		return 8 // pkg/sftp's default is 64
	}
	return cfg.MaxConnectionsPerFile
}

func (cfg *SFTP) PacketSize() int {
	if cfg == nil || cfg.MaxPacketSize == 0 {
		return 20480
	}
	return cfg.MaxPacketSize
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

type Inbound struct {
	Interval time.Duration `yaml:"interval" json:"interval"`
}

type Transfers struct {
	BalanceEntries bool     `yaml:"balance_entries" json:"balance_entries"`
	Addendum       Addendum `yaml:"addendum" json:"addendum"`
}

type Addendum struct {
	Create05 bool `yaml:"create05" json:"create05"`
}

type Storage struct {
	// CleanupLocalDirectory determines if we delete the local directory after
	// processing is finished. Leaving these files around helps debugging, but
	// also exposes customer information.
	CleanupLocalDirectory bool `yaml:"cleanup_local_directory" json:"cleanup_local_directory"`

	// KeepRemoteFiles determines if we delete the remote file on an ODFI's server
	// after downloading and processing of each file.
	KeepRemoteFiles bool `yaml:"keep_remote_files" json:"keep_remote_files"`

	Local *Local `json:"local"`
}

type Local struct {
	Directory string `yaml:"directory" json:"directory"`
}

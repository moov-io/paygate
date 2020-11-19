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
	DefaultFilenameTemplate = `{{ date "20060102" }}-{{ .RoutingNumber }}.ach{{ if .GPG }}.gpg{{ end }}`
)

// ODFI holds all the configuration for sending and retrieving ACH files with
// a financial institution to originate files.
type ODFI struct {
	// RoutingNumber is a valid ABA routing number
	RoutingNumber string

	// Gateway holds FileHeader information which the ODFI requires is set
	// on all files uploaded.
	Gateway Gateway

	Cutoffs Cutoffs

	InboundPath  string
	OutboundPath string
	ReturnPath   string

	// AllowedIPs is a comma separated list of IP addresses and CIDR ranges
	// where connections are allowed. If this value is non-empty remote servers
	// not within these ranges will not be connected to.
	AllowedIPs string

	OutboundFilenameTemplate string

	FTP  *FTP
	SFTP *SFTP

	Inbound Inbound

	FileConfig FileConfig

	Storage *Storage
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
		return fmt.Errorf("odfi config: %v", err)
	}
	if err := cfg.Cutoffs.Validate(); err != nil {
		return fmt.Errorf("odfi config: %v", err)
	}
	if err := cfg.FileConfig.Validate(); err != nil {
		return fmt.Errorf("odfi config: %v", err)
	}
	return nil
}

type Gateway struct {
	Origin          string
	OriginName      string
	Destination     string
	DestinationName string
}

type Cutoffs struct {
	Timezone string
	Windows  []string
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
	Hostname string
	Username string
	Password string

	CAFilepath   string
	DialTimeout  time.Duration
	DisabledEPSV bool
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
	Hostname string
	Username string

	Password         string
	ClientPrivateKey string
	HostPublicKey    string

	DialTimeout           time.Duration
	MaxConnectionsPerFile int
	MaxPacketSize         int
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
	Interval time.Duration
}

type FileConfig struct {
	BatchHeader BatchHeader

	BalanceEntries bool
	Addendum       Addendum
}

func (cfg FileConfig) Validate() error {
	if err := cfg.BatchHeader.Validate(); err != nil {
		return fmt.Errorf("file config: %v", err)
	}
	return nil
}

type BatchHeader struct {
	CompanyIdentification string
}

func (cfg BatchHeader) Validate() error {
	if cfg.CompanyIdentification == "" {
		return errors.New("missing companyIdentification")
	}
	return nil
}

type Addendum struct {
	Create05 bool
}

type Storage struct {
	// CleanupLocalDirectory determines if we delete the local directory after
	// processing is finished. Leaving these files around helps debugging, but
	// also exposes customer information.
	CleanupLocalDirectory bool

	// KeepRemoteFiles determines if we delete the remote file on an ODFI's server
	// after downloading and processing of each file.
	KeepRemoteFiles bool

	// RemoveZeroByteFilesAfter The amount of time to wait before deleting zero byte files.
	RemoveZeroByteFilesAfter time.Duration

	Local *Local
}

type Local struct {
	Directory string
}

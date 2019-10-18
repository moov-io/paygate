// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	LogFormat string `yaml:"log_format"`
	Database  string `yaml:"database"`

	Accounts *AccountsConfig `yaml:"accounts"`
	ACH      *ACHConfig      `yaml:"ach"`
	FED      *FEDConfig      `yaml:"fed"`
	FTP      *FTPConfig      `yaml:"ftp"`
	HTTP     *HTTPConfig     `yaml:"http"`
	HTTPS    *HTTPSConfig    `yaml:"https"`
	MySQL    *MySQLConfig    `yaml:"mysql"`
	ODFI     *ODFIConfig     `yaml:"odfi"`
	OFAC     *OFACConfig     `yaml:"ofac"`
	SFTP     *SFTPConfig     `yaml:"sftp"`
	Sqlite   *SqliteConfig   `yaml:"sqlite"`
}

func ReadConfig(path string) (*Config, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %v", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(bs, &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal %s: %v", path, err)
	}
	return &cfg, nil
}

type AccountsConfig struct {
	Disabled bool   `yaml:disabled"`
	Endpoint string `yaml:"endpoint"`
}

type ACHConfig struct {
	Endpoint  string `yaml:"endpoint"`
	BatchSize int    `yaml:"batch_size"`
	MaxLines  int    `yaml:"max_lines"`

	TransfersCAFile   string        `yaml:"transfers_ca_file"`
	TransfersInterval time.Duration `yaml:"transfers_interval"`

	StorageDir              string        `yaml:"storage_dir"`
	ForcedCutoffUploadDelta time.Duration `yaml:"forced_cutoff_upload_delta"`
}

type FEDConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type FTPConfig struct {
	DialTimeout time.Duration `yaml:"dial_timeout"`
	DisableESPV bool          `yaml:"disable_espv"`
}

type HTTPConfig struct {
	Bind         string `yaml:"bind"`
	ClientCAFile string `yaml:"client_ca_file"`
}

type HTTPSConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type MySQLConfig struct {
	Hostname       string        `yaml:"hostname"`
	Port           int           `yaml:"port"`
	Protocol       string        `yaml:"protocol"`
	Database       string        `yaml:"database"`
	Password       string        `yaml:"password"`
	User           string        `yaml:"user"`
	Timeout        time.Duration `yaml:"timeout"`
	StartupTimeout time.Duration `yaml:"startup_timeout"`
}

type ODFIConfig struct {
	AccountNumber  string `yaml:"account_number"`
	AccountType    string `yaml:"account_type"`
	BankName       string `yaml:"bank_name"`
	Holder         string `yaml:"holder"`
	Identification string `yaml:"identification"`
	RoutingNumber  string `yaml:"routing_number"`
}

type OFACConfig struct {
	Endpoint       string  `yaml:"endpoint"`
	MatchThreshold float64 `yaml:"match_threshold"`
}

type SFTPConfig struct {
	DialTimeout           time.Duration `yaml:"dial_timeout"`
	MaxConnectionsPerFile int           `yaml:"max_connections_per_file"`
	MaxPacketSize         int           `yaml:"max_packet_size"`
}

type SqliteConfig struct {
	Path string `yaml:"path"`
}

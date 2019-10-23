// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	LogFormat    string `yaml:"log_format"`
	DatabaseType string `yaml:"database_type"`

	Accounts *AccountsConfig `yaml:"accounts"`
	ACH      *ACHConfig      `yaml:"ach"`
	FED      *FEDConfig      `yaml:"fed"`
	FTP      *FTPConfig      `yaml:"ftp"`
	MySQL    *MySQLConfig    `yaml:"mysql"`
	ODFI     *ODFIConfig     `yaml:"odfi"`
	OFAC     *OFACConfig     `yaml:"ofac"`
	SFTP     *SFTPConfig     `yaml:"sftp"`
	Sqlite   *SqliteConfig   `yaml:"sqlite"`
	Web      *WebConfig      `yaml:"web"`
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

type WebConfig struct {
	BindAddress      string `yaml:"bind_address"`
	AdminBindAddress string `yaml:"admin_bind_address"`
	ClientCAFile     string `yaml:"client_ca_file"`
	CertFile         string `yaml:"cert_file"`
	KeyFile          string `yaml:"key_file"`
}

func Empty() *Config {
	cfg := Config{}
	cfg.Accounts = &AccountsConfig{}
	cfg.ACH = &ACHConfig{}
	cfg.FED = &FEDConfig{}
	cfg.FTP = &FTPConfig{}
	cfg.MySQL = &MySQLConfig{}
	cfg.ODFI = &ODFIConfig{}
	cfg.OFAC = &OFACConfig{}
	cfg.SFTP = &SFTPConfig{}
	cfg.Sqlite = &SqliteConfig{}
	cfg.Web = &WebConfig{}

	return &cfg
}

func LoadConfig(path string) (*Config, error) {
	var cfg Config

	if path != "" {
		bs, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("config: read %s: %v", path, err)
		}

		if err := yaml.Unmarshal(bs, &cfg); err != nil {
			return nil, fmt.Errorf("config: unmarshal %s: %v", path, err)
		}
	}

	err := OverrideWithEnvVars(&cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func OverrideWithEnvVars(cfg *Config) error {
	var err error

	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.LogFormat = v
	}
	if v := os.Getenv("DATABASE_TYPE"); v != "" {
		cfg.DatabaseType = v
	}

	if v := os.Getenv("ACCOUNTS_ENDPOINT"); v != "" {
		cfg.Accounts.Endpoint = v
	}
	if v := os.Getenv("ACCOUNTS_CALLS_DISABLED"); v != "" {
		cfg.Accounts.Disabled, err = strconv.ParseBool(v)
	}

	if v := os.Getenv("ACH_ENDPOINT"); v != "" {
		cfg.ACH.Endpoint = v
	}
	if v := os.Getenv("ACH_FILE_BATCH_SIZE"); v != "" {
		cfg.ACH.BatchSize, err = strconv.Atoi(v)
	}
	if v := os.Getenv("ACH_FILE_MAX_LINES"); v != "" {
		cfg.ACH.MaxLines, err = strconv.Atoi(v)
	}
	if v := os.Getenv("ACH_FILE_TRANSFERS_CAFILE"); v != "" {
		cfg.ACH.TransfersCAFile = v
	}
	if v := os.Getenv("ACH_FILE_TRANSFER_INTERVAL"); v != "" {
		cfg.ACH.TransfersInterval, err = time.ParseDuration(v)
	}
	if v := os.Getenv("ACH_FILE_STORAGE_DIR"); v != "" {
		cfg.ACH.StorageDir = v
	}
	if v := os.Getenv("FORCED_CUTOFF_UPLOAD_DELTA"); v != "" {
		cfg.ACH.ForcedCutoffUploadDelta, err = time.ParseDuration(v)
	}

	if v := os.Getenv("FED_ENDPOINT"); v != "" {
		cfg.FED.Endpoint = v
	}

	if v := os.Getenv("MYSQL_HOSTNAME"); v != "" {
		cfg.MySQL.Hostname = v
	}
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		cfg.MySQL.Port, err = strconv.Atoi(v)
	}
	if v := os.Getenv("MYSQL_PROTOCOL"); v != "" {
		cfg.MySQL.Protocol = v
	}
	if v := os.Getenv("MYSQL_DATABASE"); v != "" {
		cfg.MySQL.Database = v
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		cfg.MySQL.Password = v
	}
	if v := os.Getenv("MYSQL_USER"); v != "" {
		cfg.MySQL.User = v
	}
	if v := os.Getenv("MYSQL_TIMEOUT"); v != "" {
		cfg.MySQL.Timeout, err = time.ParseDuration(v)
	}
	if v := os.Getenv("MYSQL_STARTUP_TIMEOUT"); v != "" {
		cfg.MySQL.StartupTimeout, err = time.ParseDuration(v)
	}

	if v := os.Getenv("ODFI_ACCOUNT_NUMBER"); v != "" {
		cfg.ODFI.AccountNumber = v
	}
	if v := os.Getenv("ODFI_ACCOUNT_TYPE"); v != "" {
		cfg.ODFI.AccountType = v
	}
	if v := os.Getenv("ODFI_BANK_NAME"); v != "" {
		cfg.ODFI.BankName = v
	}
	if v := os.Getenv("ODFI_HOLDER"); v != "" {
		cfg.ODFI.Holder = v
	}
	if v := os.Getenv("ODFI_IDENTIFICATION"); v != "" {
		cfg.ODFI.Identification = v
	}
	if v := os.Getenv("ODFI_ROUTING_NUMBER"); v != "" {
		cfg.ODFI.RoutingNumber = v
	}

	if v := os.Getenv("OFAC_ENDPOINT"); v != "" {
		cfg.OFAC.Endpoint = v
	}
	if v := os.Getenv("OFAC_MATCH_THRESHOLD"); v != "" {
		cfg.OFAC.MatchThreshold, err = strconv.ParseFloat(v, 64)
	}

	if v := os.Getenv("SQLITE_DB_PATH"); v != "" {
		cfg.Sqlite.Path = v
	}

	if v := os.Getenv("HTTP_BIND_ADDRESS"); v != "" {
		cfg.Web.BindAddress = v
	}
	if v := os.Getenv("HTTP_ADMIN_BIND_ADDRESS"); v != "" {
		cfg.Web.AdminBindAddress = v
	}
	if v := os.Getenv("HTTP_CLIENT_CAFILE"); v != "" {
		cfg.Web.ClientCAFile = v
	}
	if v := os.Getenv("HTTPS_CERT_FILE"); v != "" {
		cfg.Web.CertFile = v
	}
	if v := os.Getenv("HTTPS_KEY_FILE"); v != "" {
		cfg.Web.KeyFile = v
	}

	return err
}

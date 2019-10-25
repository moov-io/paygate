// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"gopkg.in/yaml.v2"
)

// TODO(adam): add root log.Logger on Config struct (for use everywhere)
// Empty() uses log.NewNopLogger()
//
// TODO(adam): refactor params provided everywhere to instead read Config struct

type Config struct {
	Logger       log.Logger
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
	Disabled bool   `yaml:"disabled"`
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
	cfg.Logger = log.NewNopLogger()
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

func LoadConfig(path string, logFormat *string) (*Config, error) {
	cfg := Empty()

	if path != "" {
		bs, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("config: read %s: %v", path, err)
		}

		if err := yaml.Unmarshal(bs, cfg); err != nil {
			return nil, fmt.Errorf("config: unmarshal %s: %v", path, err)
		}
	}

	err := OverrideWithEnvVars(cfg)
	if err != nil {
		return nil, err
	}

	// Setup our Logger
	if *logFormat != "" {
		cfg.LogFormat = *logFormat
	}
	if strings.EqualFold(cfg.LogFormat, "json") {
		cfg.Logger = log.NewJSONLogger(os.Stderr)
	} else {
		cfg.Logger = log.NewLogfmtLogger(os.Stderr)
	}
	cfg.Logger = log.With(cfg.Logger, "ts", log.DefaultTimestampUTC)
	cfg.Logger = log.With(cfg.Logger, "caller", log.DefaultCaller)

	return cfg, nil
}

func override(env string, field *string) {
	if v := os.Getenv(env); v != "" {
		*field = v
	}
}

func OverrideWithEnvVars(cfg *Config) error {
	var err error

	override(os.Getenv("LOG_FORMAT"), &cfg.LogFormat)
	override(os.Getenv("DATABASE_TYPE"), &cfg.DatabaseType)

	override(os.Getenv("ACCOUNTS_ENDPOINT"), &cfg.Accounts.Endpoint)
	if v := os.Getenv("ACCOUNTS_CALLS_DISABLED"); v != "" {
		cfg.Accounts.Disabled, err = strconv.ParseBool(v)
	}

	override(os.Getenv("ACH_ENDPOINT"), &cfg.ACH.Endpoint)
	if v := os.Getenv("ACH_FILE_BATCH_SIZE"); v != "" {
		cfg.ACH.BatchSize, err = strconv.Atoi(v)
	}
	if v := os.Getenv("ACH_FILE_MAX_LINES"); v != "" {
		cfg.ACH.MaxLines, err = strconv.Atoi(v)
	}
	override(os.Getenv("ACH_FILE_TRANSFERS_CAFILE"), &cfg.ACH.TransfersCAFile)
	if v := os.Getenv("ACH_FILE_TRANSFER_INTERVAL"); v != "" {
		cfg.ACH.TransfersInterval, err = time.ParseDuration(v)
	}
	override(os.Getenv("ACH_FILE_STORAGE_DIR"), &cfg.ACH.StorageDir)
	if v := os.Getenv("FORCED_CUTOFF_UPLOAD_DELTA"); v != "" {
		cfg.ACH.ForcedCutoffUploadDelta, err = time.ParseDuration(v)
	}

	override(os.Getenv("FED_ENDPOINT"), &cfg.FED.Endpoint)

	override(os.Getenv("MYSQL_HOSTNAME"), &cfg.MySQL.Hostname)
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		cfg.MySQL.Port, err = strconv.Atoi(v)
	}
	override(os.Getenv("MYSQL_PROTOCOL"), &cfg.MySQL.Protocol)
	override(os.Getenv("MYSQL_DATABASE"), &cfg.MySQL.Database)
	override(os.Getenv("MYSQL_PASSWORD"), &cfg.MySQL.Password)
	override(os.Getenv("MYSQL_USER"), &cfg.MySQL.User)
	if v := os.Getenv("MYSQL_TIMEOUT"); v != "" {
		cfg.MySQL.Timeout, err = time.ParseDuration(v)
	}
	if v := os.Getenv("MYSQL_STARTUP_TIMEOUT"); v != "" {
		cfg.MySQL.StartupTimeout, err = time.ParseDuration(v)
	}

	override(os.Getenv("ODFI_ACCOUNT_NUMBER"), &cfg.ODFI.AccountNumber)
	override(os.Getenv("ODFI_ACCOUNT_TYPE"), &cfg.ODFI.AccountType)
	override(os.Getenv("ODFI_BANK_NAME"), &cfg.ODFI.BankName)
	override(os.Getenv("ODFI_HOLDER"), &cfg.ODFI.Holder)
	override(os.Getenv("ODFI_IDENTIFICATION"), &cfg.ODFI.Identification)
	override(os.Getenv("ODFI_ROUTING_NUMBER"), &cfg.ODFI.RoutingNumber)

	override(os.Getenv("OFAC_ENDPOINT"), &cfg.OFAC.Endpoint)
	if v := os.Getenv("OFAC_MATCH_THRESHOLD"); v != "" {
		cfg.OFAC.MatchThreshold, err = strconv.ParseFloat(v, 64)
	}

	override(os.Getenv("SQLITE_DB_PATH"), &cfg.Sqlite.Path)

	override(os.Getenv("HTTP_BIND_ADDRESS"), &cfg.Web.BindAddress)
	override(os.Getenv("HTTP_ADMIN_BIND_ADDRESS"), &cfg.Web.AdminBindAddress)
	override(os.Getenv("HTTP_CLIENT_CAFILE"), &cfg.Web.ClientCAFile)
	override(os.Getenv("HTTPS_CERT_FILE"), &cfg.Web.CertFile)
	override(os.Getenv("HTTPS_KEY_FILE"), &cfg.Web.KeyFile)

	return err
}

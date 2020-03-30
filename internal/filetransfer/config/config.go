// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"bytes"
	"database/sql"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	RoutingNumber string `json:"routingNumber" yaml:"routingNumber"`

	InboundPath  string `json:"inboundPath" yaml:"inboundPath"`
	OutboundPath string `json:"outboundPath" yaml:"outboundPath"`
	ReturnPath   string `json:"returnPath" yaml:"returnPath"`

	OutboundFilenameTemplate string `json:"outboundFilenameTemplate" yaml:"outboundFilenameTemplate"`

	AllowedIPs string
}

func (cfg *Config) FilenameTemplate() string {
	if cfg == nil || cfg.OutboundFilenameTemplate == "" {
		return DefaultFilenameTemplate
	}
	return cfg.OutboundFilenameTemplate
}

type Repository interface {
	GetConfigs() ([]*Config, error)
	upsertConfig(cfg *Config) error
	deleteConfig(routingNumber string) error

	GetCutoffTimes() ([]*CutoffTime, error)
	upsertCutoffTime(routingNumber string, cutoff int, loc *time.Location) error
	deleteCutoffTime(routingNumber string) error

	GetFTPConfigs() ([]*FTPConfig, error)
	upsertFTPConfigs(routingNumber, host, user, pass string) error
	deleteFTPConfig(routingNumber string) error

	GetSFTPConfigs() ([]*SFTPConfig, error)
	upsertSFTPConfigs(routingNumber, host, user, pass, privateKey, publicKey string) error
	deleteSFTPConfig(routingNumber string) error

	Close() error
}

func NewRepository(filepath string, db *sql.DB, dbType string) Repository {
	if db == nil {
		repo := &StaticRepository{}
		repo.Populate()
		return repo
	}

	// If we've got a config from a file on the filesystem let's use that
	if filepath != "" {
		repo, _ := readConfigFile(filepath)
		return repo
	}

	sqliteRepo := &SQLRepository{db}

	if strings.EqualFold(dbType, "sqlite") || strings.EqualFold(dbType, "mysql") {
		// On 'mysql' database setups return that over the local (hardcoded) values.
		return sqliteRepo
	}

	cutoffCount, ftpCount, fileTransferCount := sqliteRepo.GetCounts()
	if (cutoffCount + ftpCount + fileTransferCount) == 0 {
		repo := &StaticRepository{}
		repo.Populate()
		return repo
	}

	return sqliteRepo
}

var (
	devFileTransferType = os.Getenv("DEV_FILE_TRANSFER_TYPE")
)

func readConfigFile(path string) (Repository, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	type wrapper struct {
		FileTransfer struct {
			Configs     []*Config     `yaml:"configs"`
			CutoffTimes []*CutoffTime `yaml:"cutoffTimes"`
			FTPConfigs  []*FTPConfig  `yaml:"ftpConfigs"`
			SFTPConfigs []*SFTPConfig `yaml:"sftpConfigs"`
		} `yaml:"fileTransfer"`
	}

	var conf wrapper
	if err := yaml.NewDecoder(bytes.NewReader(bs)).Decode(&conf); err != nil {
		return nil, err
	}
	return &StaticRepository{
		Configs:     conf.FileTransfer.Configs,
		CutoffTimes: conf.FileTransfer.CutoffTimes,
		FTPConfigs:  conf.FileTransfer.FTPConfigs,
		SFTPConfigs: conf.FileTransfer.SFTPConfigs,
		Protocol:    devFileTransferType,
	}, nil
}

func readFileTransferConfig(repo Repository, routingNumber string) *Config {
	configs, err := repo.GetConfigs()
	if err != nil {
		return &Config{}
	}
	for i := range configs {
		if configs[i].RoutingNumber == routingNumber {
			return configs[i]
		}
	}
	return &Config{}
}

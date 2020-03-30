// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"strings"
	"time"
)

type StaticRepository struct {
	Configs     []*Config
	CutoffTimes []*CutoffTime
	FTPConfigs  []*FTPConfig
	SFTPConfigs []*SFTPConfig

	// Protocol represents values like ftp or sftp to return back relevant configs
	// to the moov/fsftp or SFTP docker image
	Protocol string

	Err error
}

func (r *StaticRepository) Populate() {
	r.populateConfigs()
	r.populateCutoffTimes()

	switch strings.ToLower(r.Protocol) {
	case "", "ftp":
		r.populateFTPConfigs()
	case "sftp":
		r.populateSFTPConfigs()
	}
}

func (r *StaticRepository) populateConfigs() {
	cfg := &Config{RoutingNumber: "121042882"} // test value, matches apitest

	switch strings.ToLower(r.Protocol) {
	case "", "ftp":
		// For 'make start-ftp-server', configs match paygate's testdata/ftp-server/
		cfg.InboundPath = "inbound/"
		cfg.OutboundPath = "outbound/"
		cfg.ReturnPath = "returned/"
	case "sftp":
		// For 'make start-sftp-server', configs match paygate's testdata/sftp-server/
		cfg.InboundPath = "/upload/inbound/"
		cfg.OutboundPath = "/upload/outbound/"
		cfg.ReturnPath = "/upload/returned/"
	}

	r.Configs = append(r.Configs, cfg)
}

func (r *StaticRepository) populateCutoffTimes() {
	nyc, _ := time.LoadLocation("America/New_York")
	r.CutoffTimes = append(r.CutoffTimes, &CutoffTime{
		RoutingNumber: "121042882",
		Cutoff:        1700,
		Loc:           nyc,
	})
}

func (r *StaticRepository) populateFTPConfigs() {
	r.FTPConfigs = append(r.FTPConfigs, &FTPConfig{
		RoutingNumber: "121042882",
		Hostname:      "localhost:2121", // below configs for moov/fsftp:v0.1.0
		Username:      "admin",
		Password:      "123456",
	})
}

func (r *StaticRepository) populateSFTPConfigs() {
	r.SFTPConfigs = append(r.SFTPConfigs, &SFTPConfig{
		RoutingNumber: "121042882",
		Hostname:      "localhost:2222", // below configs for atmoz/sftp:latest
		Username:      "demo",
		Password:      "password",
		// ClientPrivateKey: "...", // Base64 encoded or PEM format
	})
}

func (r *StaticRepository) GetConfigs() ([]*Config, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Configs, nil
}

func (r *StaticRepository) GetCutoffTimes() ([]*CutoffTime, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.CutoffTimes, nil
}

func (r *StaticRepository) GetFTPConfigs() ([]*FTPConfig, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.FTPConfigs, nil
}

func (r *StaticRepository) GetSFTPConfigs() ([]*SFTPConfig, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.SFTPConfigs, nil
}

func (r *StaticRepository) Close() error {
	return nil
}

func (r *StaticRepository) upsertConfig(cfg *Config) error {
	if r.Err != nil {
		return r.Err
	}
	return nil
}

func (r *StaticRepository) deleteConfig(routingNumber string) error {
	if r.Err != nil {
		return r.Err
	}
	return nil
}

func (r *StaticRepository) upsertCutoffTime(routingNumber string, cutoff int, loc *time.Location) error {
	if r.Err != nil {
		return r.Err
	}
	return nil
}

func (r *StaticRepository) deleteCutoffTime(routingNumber string) error {
	if r.Err != nil {
		return r.Err
	}
	return nil
}

func (r *StaticRepository) upsertFTPConfigs(routingNumber, host, user, pass string) error {
	if r.Err != nil {
		return r.Err
	}
	return nil
}

func (r *StaticRepository) deleteFTPConfig(routingNumber string) error {
	if r.Err != nil {
		return r.Err
	}
	return nil
}

func (r *StaticRepository) upsertSFTPConfigs(routingNumber, host, user, pass, privateKey, publicKey string) error {
	if r.Err != nil {
		return r.Err
	}
	return nil
}

func (r *StaticRepository) deleteSFTPConfig(routingNumber string) error {
	if r.Err != nil {
		return r.Err
	}
	return nil
}

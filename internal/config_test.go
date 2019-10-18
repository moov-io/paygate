// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"path/filepath"
	"testing"
	"time"
)

func TestConfig__Read(t *testing.T) {
	cfg, err := ReadConfig(filepath.Join("..", "testdata", "config-good.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.LogFormat != "json" {
		t.Errorf("cfg.LogFormat=%s", cfg.LogFormat)
	}
	if cfg.Database != "sqlite" {
		t.Errorf("cfg.Database=%s", cfg.Database)
	}

	if cfg.Accounts.Disabled != false {
		t.Errorf("cfg.Accounts.Disabled=%v", cfg.Accounts.Disabled)
	}
	if cfg.Accounts.Endpoint != "http://accounts:8080" {
		t.Errorf("cfg.Accounts.Endpoint=%v", cfg.Accounts.Endpoint)
	}

	if cfg.ACH.Endpoint != "http://ach:8080" {
		t.Errorf("cfg.ACH.Endpoint=%v", cfg.ACH.Endpoint)
	}
	if cfg.ACH.BatchSize != 100 {
		t.Errorf("cfg.ACH.BatchSize=%v", cfg.ACH.BatchSize)
	}
	if cfg.ACH.MaxLines != 1000 {
		t.Errorf("cfg.ACH.MaxLines=%v", cfg.ACH.MaxLines)
	}
	if cfg.ACH.TransfersCAFile != "/opt/paygate/ca.crt" {
		t.Errorf("cfg.ACH.TransfersCAFile=%v", cfg.ACH.TransfersCAFile)
	}
	if cfg.ACH.StorageDir != "/opt/paygate/storage/" {
		t.Errorf("cfg.ACH.StorageDir=%v", cfg.ACH.StorageDir)
	}
	if cfg.ACH.ForcedCutoffUploadDelta != 5*time.Minute {
		t.Errorf("cfg.ACH.ForcedCutoffUploadDelta=%v", cfg.ACH.ForcedCutoffUploadDelta)
	}

	if cfg.FED.Endpoint != "http://fed:8080" {
		t.Errorf("cfg.FED.Endpoint=%s", cfg.FED.Endpoint)
	}

	if cfg.FTP.DialTimeout != 10*time.Second {
		t.Errorf("cfg.FTP.DialTimeout=%v", cfg.FTP.DialTimeout)
	}
	if cfg.FTP.DisableESPV != false {
		t.Errorf("cfg.FTP.DisableESPV=%v", cfg.FTP.DisableESPV)
	}

	if cfg.HTTP.Bind != "0.0.0.0:8080" {
		t.Errorf("cfg.HTTP.Bind=%v", cfg.HTTP.Bind)
	}
	if cfg.HTTP.ClientCAFile != "/opt/paygate/client.crt" {
		t.Errorf("cfg.HTTP.ClientCAFile=%v", cfg.HTTP.ClientCAFile)
	}

	if cfg.HTTPS.CertFile != "/opt/paygate/server.crt" {
		t.Errorf("cfg.HTTPS.CertFile=%v", cfg.HTTPS.CertFile)
	}
	if cfg.HTTPS.KeyFile != "/opt/paygate/server.key" {
		t.Errorf("cfg.HTTPS.KeyFile=%v", cfg.HTTPS.KeyFile)
	}

	if cfg.MySQL.Hostname != "localhost" {
		t.Errorf("cfg.MySQL.Hostname=%v", cfg.MySQL.Hostname)
	}
	if cfg.MySQL.Port != 3306 {
		t.Errorf("cfg.MySQL.Port=%v", cfg.MySQL.Port)
	}
	if cfg.MySQL.Protocol != "tcp" {
		t.Errorf("cfg.MySQL.Protocol=%v", cfg.MySQL.Protocol)
	}
	if cfg.MySQL.Database != "paygate" {
		t.Errorf("cfg.MySQL.Database=%v", cfg.MySQL.Database)
	}
	if cfg.MySQL.Password != "secret" {
		t.Errorf("cfg.MySQL.Password=%v", cfg.MySQL.Password)
	}
	if cfg.MySQL.User != "paygate" {
		t.Errorf("cfg.MySQL.User=%v", cfg.MySQL.User)
	}
	if cfg.MySQL.Timeout != 10*time.Second {
		t.Errorf("cfg.MySQL.Timeout=%v", cfg.MySQL.Timeout)
	}
	if cfg.MySQL.StartupTimeout != 15*time.Second {
		t.Errorf("cfg.MySQL.StartupTimeout=%v", cfg.MySQL.StartupTimeout)
	}

	if cfg.ODFI.AccountNumber != "214115514" {
		t.Errorf("cfg.ODFI.AccountNumber=%v", cfg.ODFI.AccountNumber)
	}
	if cfg.ODFI.AccountType != "checking" {
		t.Errorf("cfg.ODFI.AccountType=%v", cfg.ODFI.AccountType)
	}
	if cfg.ODFI.BankName != "Moov" {
		t.Errorf("cfg.ODFI.BankName=%v", cfg.ODFI.BankName)
	}
	if cfg.ODFI.Holder != "Jane Smith" {
		t.Errorf("cfg.ODFI.Holder=%v", cfg.ODFI.Holder)
	}
	if cfg.ODFI.Identification != "29813754" {
		t.Errorf("cfg.ODFI.Identification=%v", cfg.ODFI.Identification)
	}
	if cfg.ODFI.RoutingNumber != "987654320" {
		t.Errorf("cfg.ODFI.RoutingNumber=%v", cfg.ODFI.RoutingNumber)
	}

	if cfg.OFAC.Endpoint != "http://ofac:8080" {
		t.Errorf("cfg.OFAC.Endpoint=%v", cfg.OFAC.Endpoint)
	}
	if (cfg.OFAC.MatchThreshold - 0.99) > 0.01 {
		t.Errorf("cfg.OFAC.MatchThreshold=%v", cfg.OFAC.MatchThreshold)
	}

	if cfg.SFTP.DialTimeout != 10*time.Second {
		t.Errorf("cfg.SFTP.DialTimeout=%v", cfg.SFTP.DialTimeout)
	}
	if cfg.SFTP.MaxConnectionsPerFile != 2 {
		t.Errorf("cfg.SFTP.MaxConnectionsPerFile=%v", cfg.SFTP.MaxConnectionsPerFile)
	}
	if cfg.SFTP.MaxPacketSize != 65535 {
		t.Errorf("cfg.SFTP.MaxPacketSize=%v", cfg.SFTP.MaxPacketSize)
	}

	if cfg.Sqlite.Path != "/opt/paygate/paygate.db" {
		t.Errorf("cfg.Sqlite.Path=%v", cfg.Sqlite.Path)
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	cfg, err := FromFile(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Logger == nil {
		t.Fatal("nil Logger")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("cfg.Logging.Format=%s", cfg.Logging.Format)
	}

	if cfg.ODFI.RoutingNumber != "987654320" {
		t.Errorf("ODFIConfig=%#v", cfg.ODFI)
	}
}

func TestInvalidConfig(t *testing.T) {
	cfg, err := FromFile(filepath.Join("testdata", "invalid.yaml"))
	if err == nil {
		t.Error("expected error")
	}

	if err := cfg.Validate(); err == nil {
		t.Error("expected error")
	}
}

func TestReadConfig(t *testing.T) {
	conf := []byte(`logging:
  format: plain
customers:
  endpoint: "http://localhost:8087"
  accounts:
    decryptor:
      symmetric:
        keyURI: 'MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI='
odfi:
  routingNumber: "987654320"
  gateway:
    origin: "CUSTID"
  inboundPath: "/files/inbound/"
  outboundPath: "/files/outbound/"
  returnPath: "/files/return/"
  allowedIPs: "10.1.0.1,10.2.0.0/16"
  cutoffs:
    timezone: "America/New_York"
    windows: ["17:00"]
  ftp:
    hostname: sftp.moov.io
    username: moov
    password: secret
  fileConfig:
    batchHeader:
      companyIdentification: "MoovZZZZZZ"
  storage:
    keepRemoteFiles: false
    local:
      directory: "/opt/moov/storage/"
validation:
  microDeposits:
    source:
      customerID: "user"
      accountID: "acct"
      organization: "moov"
pipeline:
  merging:
    directory: "./storage/"
  stream:
    inmem:
      url: "mem://paygate"
`)
	cfg, err := Read(conf)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Error(err)
	}

	if cfg == nil {
		t.Fatal("nil Config")
	}

	if cfg.Customers.Endpoint != "http://localhost:8087" {
		t.Errorf("customers endpoint: %q", cfg.Customers.Endpoint)
	}
	if cfg.Customers.Accounts.Decryptor.Symmetric.KeyURI != "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=" {
		t.Errorf("accounts decryptor KeyURI=%q", cfg.Customers.Accounts.Decryptor.Symmetric.KeyURI)
	}

	if cfg.Validation.MicroDeposits.Source.CustomerID != "user" {
		t.Errorf("unexpected verification: %#v", cfg.Validation)
	}

	if cfg.Pipeline.Stream.InMem.URL != "mem://paygate" {
		t.Errorf("missing pipeline stream config: %#v", cfg.Pipeline.Stream)
	}
}

func TestConfig__FTP(t *testing.T) {
	cfg := Empty().ODFI.FTP
	if cfg != nil {
		t.Fatalf("unexpected %#v", cfg)
	}

	if v := cfg.CAFile(); v != "" {
		t.Errorf("caFile=%q", v)
	}
	if v := cfg.Timeout(); v != 10*time.Second {
		t.Errorf("dialTimeout=%v", v)
	}
	if v := cfg.DisableEPSV(); v {
		t.Errorf("disabledEPSV=%v", v)
	}
}

func TestConfig__SFTP(t *testing.T) {
	cfg := Empty().ODFI.SFTP
	if cfg != nil {
		t.Fatalf("unexpected %#v", cfg)
	}

	if v := cfg.Timeout(); v != 10*time.Second {
		t.Errorf("dialTimeout=%v", v)
	}
	if v := cfg.MaxConnections(); v != 1 {
		t.Errorf("maxConnectionsPerFile=%d", v)
	}
	if v := cfg.PacketSize(); v != 20480 {
		t.Errorf("maxPacketSize=%d", v)
	}
}

func TestConfig__Uppercase(t *testing.T) {
	// What can be read out for a config if the case of each key varies from lowercase
	data := []byte(`Customers:
  ENDPOINT: "http://localhost:8087"
odfi:
  RoutingNumber: "987654320"
  cutoffs:
    timezone: "America/New_York"
    windows:
      - "16:20"
  OutboundPath: "/files/outbound/"
  fileConfig:
    batchHeader:
      companyIdentification: "MoovZZZZZZ"
`)
	cfg, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}

	if v := cfg.ODFI.OutboundPath; v != "/files/outbound/" {
		t.Errorf("ODFI OutboundPath: %q", v)
	}
}

func TestConfigMasked(t *testing.T) {
	cfg, err := FromFile(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Database.MySQL = &MySQL{Username: "paygate", Password: "mysql-secret"}
	cfg.ODFI.SFTP = &SFTP{Username: "moov", Password: "sftp-secret", ClientPrivateKey: "-----BEGIN KEY-----"}
	cfg.Pipeline.Notifications = &PipelineNotifications{
		Email: &Email{
			From:          "noreply@moov.io",
			To:            []string{"ops@moov.io"},
			ConnectionURI: "smtps://test:test@localhost:1025/?insecure_skip_verify=true",
			CompanyName:   "Moov",
		},
		PagerDuty: &PagerDuty{ApiKey: "pd-secret", ServiceKey: "svc-secret"},
		Slack:     &Slack{WebhookURL: "https://hooks.slack.com/services/secret"},
	}

	masked := cfg.Masked()
	if masked == nil {
		t.Fatal("expected masked config")
	}
	if cfg.ODFI.FTP.Password != "secret" {
		t.Fatalf("Masked mutated the original FTP password: %q", cfg.ODFI.FTP.Password)
	}
	if masked.ODFI.FTP.Password == "secret" || !strings.Contains(masked.ODFI.FTP.Password, "*") {
		t.Fatalf("FTP password not masked: %q", masked.ODFI.FTP.Password)
	}
	if masked.ODFI.SFTP.Password == "sftp-secret" {
		t.Fatalf("SFTP password not masked: %q", masked.ODFI.SFTP.Password)
	}
	if masked.Database.MySQL.Password == "mysql-secret" {
		t.Fatalf("MySQL password not masked: %q", masked.Database.MySQL.Password)
	}
	if masked.Customers.Accounts.Decryptor.Symmetric.KeyURI == cfg.Customers.Accounts.Decryptor.Symmetric.KeyURI {
		t.Fatal("keyURI was not masked")
	}
	if strings.Contains(masked.Pipeline.Notifications.Email.ConnectionURI, "test:test@") {
		t.Fatalf("SMTP URI credentials not masked: %q", masked.Pipeline.Notifications.Email.ConnectionURI)
	}
	if masked.Pipeline.Notifications.PagerDuty.ApiKey == "pd-secret" {
		t.Fatal("PagerDuty API key was not masked")
	}
	if masked.Pipeline.Notifications.Slack.WebhookURL == "https://hooks.slack.com/services/secret" {
		t.Fatal("Slack webhook was not masked")
	}
}

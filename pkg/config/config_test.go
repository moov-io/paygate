// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"path/filepath"
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
  storage:
    keepRemoteFiles: false
    local:
      directory: "/opt/moov/storage/"
validation:
  microDeposits:
    source:
      customerID: "user"
      accountID: "acct"
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
	if v := cfg.MaxConnections(); v != 8 {
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
`)
	cfg, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%#v\n", cfg.Customers)

	if v := cfg.ODFI.OutboundPath; v != "/files/outbound/" {
		t.Errorf("ODFI OutboundPath: %q", v)
	}
}

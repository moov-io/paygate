// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	cfg, err := FromFile(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Logger == nil {
		t.Fatal("nil Logger")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("cfg.LogFormat=%s", cfg.LogFormat)
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
	conf := []byte(`log_format: json
customers:
  endpoint: "http://localhost:8087"
  accounts:
    decryptor:
      symmetric:
        keyURI: 'MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI='
odfi:
  routing_number: "987654320"
  gateway:
    origin: "CUSTID"
  inbound_path: "/files/inbound/"
  outbound_path: "/files/outbound/"
  return_path: "/files/return/"
  allowed_ips: "10.1.0.1,10.2.0.0/16"
  cutoffs:
    timezone: "America/New_York"
    windows: ["17:00"]
  ftp:
    hostname: sftp.moov.io
    username: moov
    password: secret
  storage:
    keep_remote_files: false
    local:
      directory: "/opt/moov/storage/"
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

	if cfg.Pipeline.Stream.InMem.URL != "mem://paygate" {
		t.Errorf("missing pipeline stream config: %#v", cfg.Pipeline.Stream)
	}
}

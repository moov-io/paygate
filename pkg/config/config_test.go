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
odfi:
  routing_number: "987654320"
  gateway:
    origin: "CUSTID"
  inbound_path: "/files/inbound/"
  outbound_path: "/files/outbound/"
  return_path: "/files/return/"
  allowed_ips: "10.1.0.1,10.2.0.0/16"
  ftp:
    hostname: sftp.moov.io
    username: moov
    password: secret
  storage:
    keep_remote_files: false
    local:
      directory: "/opt/moov/storage/"
offloader:
  interval: 10m
  local:
    directory: ./storage/
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
	if cfg.Offloader.Local == nil {
		t.Errorf("missing offloader config: %#v", cfg.Offloader)
	}
}

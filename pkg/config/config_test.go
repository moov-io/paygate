// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join("testdata", "valid.yaml"))
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
	_, err := LoadConfig(filepath.Join("testdata", "invalid.yaml"))
	if err == nil {
		t.Error("expected error")
	}
}

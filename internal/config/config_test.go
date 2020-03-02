// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfig__Load(t *testing.T) {
	logFormat := "json"
	cfg, err := LoadConfig(filepath.Join("..", "..", "testdata", "configs", "valid.yaml"), &logFormat)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Logger == nil {
		t.Fatal("nil Logger")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("cfg.LogFormat=%s", cfg.LogFormat)
	}
	if cfg.Customers == nil || cfg.Customers.OFACRefreshEvery != 1440*time.Hour {
		t.Errorf("customers ofacRefreshEvery: %v", cfg.Customers.OFACRefreshEvery)
	}
}

func TestConfig__override(t *testing.T) {
	type config struct {
		Foo string
	}
	cfg := &config{Foo: "foo"}

	os.Setenv("UNIQUE_ENV_KEY_THATS_UNSET", "bar baz")
	override("UNIQUE_ENV_KEY_THATS_UNSET", &cfg.Foo)

	if cfg.Foo != "bar baz" {
		t.Errorf("cfg.Foo=%v", cfg.Foo)
	}
}

// func writeConfig(t *testing.T, raw string) string {
// 	dir, err := ioutil.TempDir("", "ach")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	path := filepath.Join(dir, "conf.yaml")
// 	if err := ioutil.WriteFile(path, []byte(raw), 0644); err != nil {
// 		t.Fatal(err)
// 	}
// 	return path
// }

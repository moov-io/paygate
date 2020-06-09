// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"testing"
)

func TestValidation(t *testing.T) {
	cfg := Validation{}
	if err := cfg.Validate(); err != nil {
		t.Error(err)
	}
}

func TestMicroDeposits(t *testing.T) {
	cfg := &MicroDeposits{
		Source: Source{},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error")
	}
}

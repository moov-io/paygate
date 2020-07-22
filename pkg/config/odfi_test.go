// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"testing"
)

func TestCutoffs_Location(t *testing.T) {
	cfg := Cutoffs{
		Timezone: "America/New_York",
	}
	if loc := cfg.Location(); loc == nil {
		t.Fatal("nil time.Location")
	}
}

func TestODFI__Validate(t *testing.T) {
	cfg := &ODFI{
		RoutingNumber: "987654320",
		Cutoffs: Cutoffs{
			Timezone: "America/New_York",
			Windows:  []string{"16:30"},
		},
		FileConfig: FileConfig{
			BatchHeader: BatchHeader{
				CompanyIdentification: "MoovZZZZZZ",
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

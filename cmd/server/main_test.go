// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"strings"
	"testing"

	"github.com/moov-io/paygate/pkg/config"
)

func TestValidateTemplate(t *testing.T) {
	cfg := config.ODFI{
		RoutingNumber: "987654320",
		Cutoffs: config.Cutoffs{
			Timezone: "America/New_York",
			Windows:  []string{"16:30"},
		},
	}
	if err := validateTemplate(cfg); err != nil {
		t.Fatal(err)
	}

	cfg.OutboundFilenameTemplate = "{{ blah }" // invalid
	if err := validateTemplate(cfg); err == nil {
		t.Fatal("expected error")
	}

	cfg.OutboundFilenameTemplate = "{{ \"\" }}"
	if err := validateTemplate(cfg); err == nil {
		t.Fatal("expected error")
	} else {
		if !strings.Contains(err.Error(), "empty filename rendered") {
			t.Error(err)
		}
	}
}

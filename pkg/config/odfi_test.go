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

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package output

import (
	"testing"

	"github.com/moov-io/paygate/pkg/config"
)

func TestFormatter(t *testing.T) {
	cfg := config.Empty()
	cfg.Pipeline = config.Pipeline{
		Output: &config.Output{
			Format: "other",
		},
	}

	enc, err := NewFormatter(cfg.Pipeline.Output)
	if err == nil {
		t.Fatal("expected error")
	}
	if enc != nil {
		t.Errorf("unexpected Formatter: %#v", enc)
	}
}

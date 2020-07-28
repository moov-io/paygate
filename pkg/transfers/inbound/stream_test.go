// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"testing"

	"github.com/moov-io/paygate/pkg/config"
)

func TestStreamEmitterErr(t *testing.T) {
	cfg := config.Empty()
	cfg.Transfers = config.Transfers{
		Inbound: config.Inbound{
			Stream: &config.StreamPipeline{
				InMem: &config.InMemPipeline{
					URL: "mem://paygate",
				},
			},
		},
	}
	pc, err := NewStreamEmitter(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Expect an error from unhandled File events
	if err := pc.Handle(File{}); err == nil {
		t.Error("expected error")
	}
}

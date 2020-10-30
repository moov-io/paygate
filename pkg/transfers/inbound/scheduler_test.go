// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"testing"
	"time"

	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/upload"
)

func TestScheduler(t *testing.T) {
	cfg := config.Empty()
	cfg.ODFI.Inbound.Interval = 10 * time.Second
	cfg.ODFI.Storage = &config.Storage{
		CleanupLocalDirectory:    true,
		KeepRemoteFiles:          false,
		RemoveZeroByteFilesAfter: 10 * time.Minute,
	}

	agent := &upload.MockAgent{}
	processors := SetupProcessors(&MockProcessor{})

	schd := NewPeriodicScheduler(cfg, agent, processors)
	if schd == nil {
		t.Fatal("nil Scheduler")
	}

	ss, ok := schd.(*PeriodicScheduler)
	if !ok {
		t.Fatalf("unexpected scheduler: %T", schd)
	}

	if err := ss.tick(); err != nil {
		t.Fatal(err)
	}
}

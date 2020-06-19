// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/upload"
)

type Scheduler interface {
	Start() error
	Shutdown()
}

type PeriodicScheduler struct {
	cfg    config.ODFI
	logger log.Logger

	ticker       *time.Ticker
	shutdown     context.Context
	shutdownFunc context.CancelFunc

	agent      upload.Agent
	downloader Downloader
	processors Processors
}

func NewPeriodicScheduler(
	cfg *config.Config,
	agent upload.Agent,
	processors Processors,
) Scheduler {
	if cfg.ODFI.Inbound.Interval == 0*time.Second {
		cfg.Logger.Log("inbound", "skipping inbound file processing")
		return &MockScheduler{}
	} else {
		cfg.Logger.Log("inbound", fmt.Sprintf("starting inbound processor with interval=%v", cfg.ODFI.Inbound.Interval))
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	return &PeriodicScheduler{
		cfg:    cfg.ODFI,
		logger: cfg.Logger,

		ticker:       time.NewTicker(cfg.ODFI.Inbound.Interval),
		shutdown:     ctx,
		shutdownFunc: cancelFunc,

		agent:      agent,
		downloader: NewDownloader(cfg.Logger, cfg.ODFI.Storage),
		processors: processors,
	}
}

func (s *PeriodicScheduler) Shutdown() {
	if s == nil {
		return
	}
	s.shutdownFunc()
}

func (s *PeriodicScheduler) Start() error {
	for {
		select {
		case <-s.ticker.C:
			if err := s.tick(); err != nil {
				s.logger.Log("inbound", fmt.Errorf("ERROR with inbound file processor: %v", err))
			}

		case <-s.shutdown.Done():
			s.logger.Log("inbound", "scheduler shutdown")
			return nil
		}
	}
}

func (s *PeriodicScheduler) tick() error {
	s.logger.Log("inbound", "start retrieving and processing of inbound files")

	dl, err := s.downloader.CopyFilesFromRemote(s.agent)
	if err != nil {
		return fmt.Errorf("ERROR: problem moving files: %v", err)
	}

	if s.cfg.Storage != nil {
		if s.cfg.Storage.CleanupLocalDirectory {
			defer dl.deleteFiles()
		} else {
			defer dl.deleteEmptyDirs(s.agent)
		}
	}

	if err := ProcessFiles(dl, s.processors); err != nil {
		return fmt.Errorf("ERROR: processing files: %v", err)
	}

	if s.cfg.Storage != nil && !s.cfg.Storage.KeepRemoteFiles {
		if err := Cleanup(s.logger, s.agent, dl); err != nil {
			return fmt.Errorf("ERROR: deleting remote files: %v", err)
		}
	}

	return nil
}

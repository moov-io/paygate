// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"github.com/moov-io/ach"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	correctionCodesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "correction_codes_processed",
		Help: "Counter of correction (COR/NOC) files processed",
	}, []string{"origin", "destination", "code"})
)

type correctionProcessor struct {
	logger log.Logger
}

func NewCorrectionProcessor(logger log.Logger) *correctionProcessor {
	return &correctionProcessor{
		logger: logger,
	}
}

func (pc *correctionProcessor) Type() string {
	return "correction"
}

func (pc *correctionProcessor) Handle(event File) error {
	if !isCorrectionFile(event.File) {
		return nil
	}
	for i := range event.File.NotificationOfChange {
		pc.logger.Log("inbound", "correction", "origin", event.File.Header.ImmediateOrigin, "destination", event.File.Header.ImmediateDestination)

		entries := event.File.NotificationOfChange[i].GetEntries()
		for j := range entries {
			if entries[j].Addenda98 == nil {
				continue
			}

			changeCode := entries[j].Addenda98.ChangeCodeField()
			correctionCodesProcessed.With(
				"origin", event.File.Header.ImmediateOrigin,
				"destination", event.File.Header.ImmediateDestination,
				"code", changeCode.Code,
			).Add(1)
		}
	}
	return nil
}

func isCorrectionFile(file *ach.File) bool {
	return (file != nil) && (len(file.NotificationOfChange) > 0)
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"github.com/moov-io/ach"

	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/moov-io/base/log"
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

func (pc *correctionProcessor) Handle(file *ach.File) error {
	if len(file.NotificationOfChange) == 0 {
		return nil
	}

	for i := range file.NotificationOfChange {
		pc.logger.With(log.Fields{
			"origin":      file.Header.ImmediateOrigin,
			"destination": file.Header.ImmediateDestination,
		}).Log("inbound: correction")

		entries := file.NotificationOfChange[i].GetEntries()
		for j := range entries {
			if entries[j].Addenda98 == nil {
				continue
			}

			changeCode := entries[j].Addenda98.ChangeCodeField()
			correctionCodesProcessed.With(
				"origin", file.Header.ImmediateOrigin,
				"destination", file.Header.ImmediateDestination,
				"code", changeCode.Code,
			).Add(1)
		}
	}

	return nil
}

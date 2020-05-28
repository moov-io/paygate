// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"errors"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/transfers"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	returnEntriesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "return_entries_processed",
		Help: "Counter of return EntryDetail records processed",
	}, []string{"origin", "destination", "code"})
)

type returnProcessor struct {
	logger log.Logger
}

func NewReturnProcessor(logger log.Logger) *returnProcessor {
	return &returnProcessor{
		logger: logger,
	}
}

func (pc *returnProcessor) Handle(file *ach.File) error {
	if len(file.ReturnEntries) == 0 {
		return nil
	}

	for i := range file.ReturnEntries {
		pc.logger.Log("inbound", "return", "origin", file.Header.ImmediateOrigin, "destination", file.Header.ImmediateDestination)

		entries := file.ReturnEntries[i].GetEntries()
		for j := range entries {
			if entries[j].Addenda99 == nil {
				continue // TODO(adam): log, moov-io/ach bug
			}

			returnCode := entries[j].Addenda99.ReturnCodeField()
			correctionCodesProcessed.With(
				"origin", file.Header.ImmediateOrigin,
				"destination", file.Header.ImmediateDestination,
				"code", returnCode.Code,
			).Add(1)

			// if err := SaveReturnCode(repo, transferID, entries[j]); err != nil
		}
	}

	return nil
}

func SaveReturnCode(repo transfers.Repository, transferID string, ed *ach.EntryDetail) error {
	if repo == nil {
		return errors.New("nil Repository")
	}
	if ed == nil || ed.Addenda99 == nil {
		return errors.New("nil ach.EntryDetail or missing Addenda99")
	}
	returnCode := ed.Addenda99.ReturnCodeField()
	if returnCode != nil {
		return repo.SetReturnCode(transferID, returnCode.Code)
	}
	return nil
}

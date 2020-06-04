// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/model"
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

	missingReturnTransfers = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "missing_return_transfers",
		Help: "Counter of return EntryDetail records handled without a found transfer",
	}, []string{"origin", "destination", "code"})
)

type returnProcessor struct {
	logger       log.Logger
	transferRepo transfers.Repository
}

func NewReturnProcessor(logger log.Logger, transferRepo transfers.Repository) *returnProcessor {
	return &returnProcessor{
		logger:       logger,
		transferRepo: transferRepo,
	}
}

func (pc *returnProcessor) Type() string {
	return "return"
}

func (pc *returnProcessor) Handle(file *ach.File) error {
	if len(file.ReturnEntries) == 0 {
		return nil
	}

	pc.logger.Log("inbound", "processing return file", "origin", file.Header.ImmediateOrigin, "destination", file.Header.ImmediateDestination)

	for i := range file.ReturnEntries {
		entries := file.ReturnEntries[i].GetEntries()
		for j := range entries {
			if entries[j].Addenda99 == nil {
				continue // TODO(adam): log, moov-io/ach bug
			}

			returnEntriesProcessed.With(
				"origin", file.Header.ImmediateOrigin,
				"destination", file.Header.ImmediateDestination,
				"code", entries[j].Addenda99.ReturnCodeField().Code,
			).Add(1)

			bh := file.ReturnEntries[i].GetHeader()
			if err := pc.processReturnEntry(file.Header, bh, entries[j]); err != nil {
				return err // TODO(adam): should we just log here?
			}
		}
	}
	return nil
}

func (pc *returnProcessor) processReturnEntry(fh ach.FileHeader, bh *ach.BatchHeader, entry *ach.EntryDetail) error {
	amount, err := model.NewAmountFromInt("USD", entry.Amount)
	if err != nil {
		return fmt.Errorf("invalid amount: %v", entry.Amount)
	}
	effectiveEntryDate, err := time.Parse("060102", bh.EffectiveEntryDate) // YYMMDD
	if err != nil {
		return fmt.Errorf("invalid EffectiveEntryDate=%q: %v", bh.EffectiveEntryDate, err)
	}

	// Do we find a Transfer related to the ach.EntryDetail?
	transfer, err := pc.transferRepo.LookupTransferFromReturn(amount, entry.TraceNumber, effectiveEntryDate)
	if transfer != nil {
		pc.logger.Log("inbound", fmt.Sprintf("handling return for transferID=%s", transfer.TransferID))
		if err := SaveReturnCode(pc.transferRepo, transfer.TransferID, entry); err != nil {
			return err
		}
		if err := pc.transferRepo.UpdateTransferStatus(transfer.TransferID, client.FAILED); err != nil {
			return fmt.Errorf("problem marking transferID=%s as %s: %v", transfer.TransferID, client.FAILED, err)
		}
		// TODO(adam): We need to update the Customer/Account as rejected based on the return code
	} else {
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("problem with returned Transfer: %v", err)
		}
		pc.logger.Log("inbound", fmt.Sprintf("transfer not found from return entry, traceNumber=%s", entry.TraceNumber))
		missingReturnTransfers.With(
			"origin", fh.ImmediateOrigin,
			"destination", fh.ImmediateDestination,
			"code", entry.Addenda99.ReturnCodeField().Code).Add(1)
	}

	// TODO(adam): lookup any micro-deposits from the transferID

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
		if err := repo.SaveReturnCode(transferID, returnCode.Code); err != nil {
			return fmt.Errorf("problem saving transferID=%s return code: %s: %v", transferID, returnCode.Code, err)
		}
	}
	return nil
}

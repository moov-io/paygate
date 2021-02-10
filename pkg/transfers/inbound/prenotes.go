// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"fmt"

	"github.com/moov-io/ach"

	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/moov-io/base/log"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	prenoteEntriesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "prenote_entries_processed",
		Help: "Counter of prenote EntryDetail records processed",
	}, []string{"origin", "destination", "transactionCode"})
)

type prenoteProcessor struct {
	logger log.Logger
}

func NewPrenoteProcessor(logger log.Logger) *prenoteProcessor {
	return &prenoteProcessor{
		logger: logger,
	}
}

func (pc *prenoteProcessor) Type() string {
	return "prenote"
}

func (pc *prenoteProcessor) Handle(file *ach.File) error {
	for i := range file.Batches {
		entries := file.Batches[i].GetEntries()
		for j := range entries {
			if ok, _ := isPrenoteEntry(entries[j]); !ok {
				continue
			}
			pc.logger.With(log.Fields{
				"origin":      log.String(file.Header.ImmediateOrigin),
				"destination": log.String(file.Header.ImmediateDestination),
			}).Log("inbound: prenote")

			prenoteEntriesProcessed.With(
				"origin", file.Header.ImmediateOrigin,
				"destination", file.Header.ImmediateDestination,
				"transactionCode", fmt.Sprintf("%d", entries[j].TransactionCode),
			).Add(1)

			// TODO(adam): We need to check our Accounts storage / GL and return the prenote
		}
	}
	return nil
}

// isPrenoteEntry checks if a given EntryDetail matches the pre-notification
// criteria. Per NACHA rules that means a zero amount and prenote transaction code.
func isPrenoteEntry(entry *ach.EntryDetail) (bool, error) {
	switch entry.TransactionCode {
	case
		ach.CheckingPrenoteCredit, ach.CheckingPrenoteDebit,
		ach.SavingsPrenoteCredit, ach.SavingsPrenoteDebit,
		ach.GLPrenoteCredit, ach.GLPrenoteDebit, ach.LoanPrenoteCredit:
		if entry.Amount == 0 {
			return true, nil // valid prenotification entry
		} else {
			return true, fmt.Errorf("non-zero prenotification amount=%d", entry.Amount)
		}
	}
	return false, nil
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"fmt"

	"github.com/moov-io/ach"
	// "github.com/go-kit/kit/metrics/prometheus"
	// stdprometheus "github.com/prometheus/client_golang/prometheus"
)

// var (
// 	prenoteFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
// 		Name: "prenote_ach_files_processed",
// 		Help: "Counter of prenote files processed",
// 	}, []string{"origin", "destination"})
// )

// inboundFilesProcessed.With( "origin", file.Header.ImmediateOrigin, "destination", file.Header.ImmediateDestination).Add(1)

// handle incoming prenote ACH files

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

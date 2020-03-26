// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"time"

	"github.com/moov-io/ach"
)

func (c *Controller) containsPrenoteEntries(req *periodicFileOperationsRequest, file *ach.File, filename string) bool {
	for i := range file.Batches {
		entries := file.Batches[i].GetEntries()
		for j := range entries {
			if ok, _ := isPrenoteEntry(entries[j]); ok {
				return true
			}
		}
	}
	return false
}

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
	default:
		return false, nil // TransactionCode isn't pre-note
	}
	return false, nil
}

func (c *Controller) processPrenoteEntries(req *periodicFileOperationsRequest, file *ach.File, filename string) error {
	for i := range file.Batches {
		entries := file.Batches[i].GetEntries()
		for j := range entries {
			if ok, err := isPrenoteEntry(entries[j]); ok {
				if err != nil {
					// TODO(adam): need to issue a return or NOC/COR for invalid prenote
					continue
				}

				// TODO(adam): check EffectiveEntryDate

				// handle prenote, lookup account number to verify if it exists or not
				dep, err := c.depRepo.LookupDepositoryFromReturn(file.Header.ImmediateDestination, entries[j].DFIAccountNumber)
				if err != nil {
					c.logger.Log(
						"processPrenoteEntries", fmt.Sprintf("problem looking up prenote account from file=%s: %v", filename, err),
						"userID", req.userID, "requestID", req.requestID)

					// TODO(adam): generate prenote return, but we should schedule a retry
				}
				if dep != nil {
					// TODO(adam): account exists, so do we need to reply?

				} else {
					// TODO(adam): no account found, so generate NOC/COR?
				}
			}
		}
	}
	return nil
}

func returnPrenoteEntry(fh ach.FileHeader, b ach.Batcher, entry *ach.EntryDetail) (*ach.File, error) {
	file := ach.NewFile()
	file.Header.ImmediateOrigin = fh.ImmediateDestination
	file.Header.ImmediateOriginName = fh.ImmediateDestinationName
	file.Header.ImmediateDestination = fh.ImmediateOrigin
	file.Header.ImmediateDestinationName = fh.ImmediateOriginName
	file.Header.FileIDModifier = "A"

	now := time.Now()
	file.Header.FileCreationDate = now.Format("060102") // YYMMDD
	file.Header.FileCreationTime = now.Format("1504")   // HHMM

	batchHeader := b.GetHeader()
	batchHeader.EffectiveEntryDate = now.Format("060102") // YYMMDD

	// entry.RDFIIdentification / .CheckDigit

	batch, err := ach.NewBatch(batchHeader)
	if err != nil {
		return nil, err
	}
	batch.AddEntry(entry)
	if err := batch.Create(); err != nil {
		return nil, err
	}

	file.AddBatch(batch)
	if err := file.Create(); err != nil {
		return nil, err
	}

	return file, nil
}

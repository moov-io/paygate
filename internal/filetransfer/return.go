// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	returnFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "return_ach_files_processed",
		Help: "Counter of return files processed",
	}, []string{"destination", "origin"})
)

func (c *fileTransferController) processReturnFiles(dir string, depRepo internal.DepositoryRepository, transferRepo internal.TransferRepository) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if (err != nil && err != filepath.SkipDir) || info.IsDir() {
			return nil // Ignore SkipDir and directories
		}

		file, err := parseACHFilepath(path)
		if err != nil {
			c.logger.Log("processReturnFiles", fmt.Sprintf("problem parsing return file %s", path), "error", err)
			return nil
		}
		c.logger.Log("processReturnFiles", fmt.Sprintf("processing return file %s from %s (%s)", info.Name(), file.Header.ImmediateOriginName, file.Header.ImmediateOrigin))

		returnFilesProcessed.With("destination", file.Header.ImmediateDestination, "origin", file.Header.ImmediateOrigin).Add(1)

		// Process each returned Batch and update their Transfer status
		//
		// We match the return file against transfers in our database and try to compare against fields
		// that can't change (and if they do it's clearly a different transfer).
		for i := range file.ReturnEntries {
			entries := file.ReturnEntries[i].GetEntries()
			for j := range entries {
				// Skip if the ach.Batch is invalid (for returns)
				if entries[j].Addenda99 == nil || entries[j].Addenda99.ReturnCodeField() == nil {
					c.logger.Log("processReturnFiles", "empty Addenda99 (or ReturnCode)", "traceNumber", entries[j].TraceNumber)
					continue
				}
				if err := c.processReturnEntry(file.Header, file.ReturnEntries[i].GetHeader(), entries[j], depRepo, transferRepo); err != nil {
					c.logger.Log("processReturnFiles", "error processing EntryDetail", "traceNumber", entries[j].TraceNumber, "error", err)
					continue
				}
			}
		}
		return nil
	})
}

func (c *fileTransferController) processReturnEntry(fileHeader ach.FileHeader, header *ach.BatchHeader, entry *ach.EntryDetail, depRepo internal.DepositoryRepository, transferRepo internal.TransferRepository) error {
	amount, err := internal.NewAmountFromInt("USD", entry.Amount)
	if err != nil {
		return fmt.Errorf("invalid amount: %v", entry.Amount)
	}
	effectiveEntryDate, err := time.Parse("060102", header.EffectiveEntryDate) // YYMMDD
	if err != nil {
		return fmt.Errorf("invalid EffectiveEntryDate=%q: %v", header.EffectiveEntryDate, err)
	}

	requestID := base.ID()
	returnCode := entry.Addenda99.ReturnCodeField()

	// Do we find a Transfer related to the ach.EntryDetail?
	transfer, err := transferRepo.LookupTransferFromReturn(header.StandardEntryClassCode, amount, entry.TraceNumber, effectiveEntryDate)
	if transfer != nil {
		if err := c.processTransferReturn(requestID, transfer, transferRepo, returnCode); err != nil {
			return fmt.Errorf("processTransferReturn: %v", err)
		}
		c.logger.Log("processReturnEntry", fmt.Sprintf("matched traceNumber=%s to transfer=%s with returnCode=%s", entry.TraceNumber, transfer.ID, returnCode), "requestID", requestID)

		// optionally update Status on Depository's related to transfer if the ReturnCode requires
		origDep, recDep, err := findDepositoriesForFileHeader(transfer.UserID, fileHeader, entry, depRepo)
		if err != nil {
			return fmt.Errorf("error finding depositories: %v", err)
		}
		c.logger.Log("processReturnEntry", fmt.Sprintf("found deposiories for transfer=%s (originator=%s) (receiver=%s)", transfer.ID, origDep.ID, recDep.ID), "requestID", requestID)

		// Optionally update the Depositories for this Transfer if the return code justifies it
		if err := updateDepositoryFromReturnCode(c.logger, returnCode, origDep, recDep, depRepo); err != nil {
			return fmt.Errorf("problem with updateDepositoryFromReturnCode transfer=%q: %v", transfer.ID, err)
		}
		return nil
	} else {
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("problem with returned Transfer: %v", err)
		}
	}

	// No Transfer, so maybe a Depository? It could be a micro-deposit.
	dep, err := depRepo.LookupDepositoryFromReturn(fileHeader.ImmediateDestination, entry.DFIAccountNumber)
	if err != nil {
		return fmt.Errorf("problem looking up Depository: %v", err)
	}
	microDeposit, err := depRepo.LookupMicroDepositFromReturn(dep.ID, amount)
	if microDeposit != nil {
		if err := c.processMicroDepositReturn(requestID, dep.UserID(), dep.ID, microDeposit, depRepo, returnCode); err != nil {
			return fmt.Errorf("processMicroDepositReturn: %v", err)
		}
		c.logger.Log("processReturnEntry", fmt.Sprintf("matched micro-deposit to depository=%s with returnCode=%s", dep.ID, returnCode), "requestID", requestID)

		// Grab the Depository objects for this micro-deposit
		origDep, recDep, err := findDepositoriesForFileHeader(dep.UserID(), fileHeader, entry, depRepo)
		if err != nil {
			return fmt.Errorf("error finding depositories: %v", err)
		}
		c.logger.Log("processReturnEntry", fmt.Sprintf("found Deposiory objects for micro-deposit (originator=%s) (receiver=%s)", origDep.ID, recDep.ID), "requestID", requestID)

		// Optionally update the Depositories for this Transfer if the return code justifies it
		if err := updateDepositoryFromReturnCode(c.logger, returnCode, origDep, recDep, depRepo); err != nil {
			return fmt.Errorf("problem with updateDepositoryFromReturnCode transfer=%q: %v", transfer.ID, err)
		}
		return nil
	} else {
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("problem with returned micro-deposit: %v", err)
		}
	}

	return fmt.Errorf("unable to match return file origin=%s traceNumber=%s", fileHeader.ImmediateOrigin, entry.TraceNumber)
}

func findDepositoriesForFileHeader(userID string, fileHeader ach.FileHeader, entry *ach.EntryDetail, depRepo internal.DepositoryRepository) (*internal.Depository, *internal.Depository, error) {
	deps, err := depRepo.GetUserDepositories(userID)
	if err != nil {
		return nil, nil, fmt.Errorf("problem finding user Depository objects: %v", err)
	}

	// Find Originator and Receiver Depository objects
	var origDep *internal.Depository
	var recDep *internal.Depository
	for k := range deps {
		if fileHeader.ImmediateOrigin == deps[k].RoutingNumber { // TODO(adam): Should we match the originator's account number?
			origDep = deps[k] // Originator Depository matched
		}
		if deps[k].RoutingNumber == fileHeader.ImmediateDestination && deps[k].AccountNumber == entry.DFIAccountNumber {
			recDep = deps[k] // Receiver Depository matched
		}
	}
	if origDep == nil || recDep == nil {
		p := func(d *internal.Depository) string {
			if d == nil {
				return ""
			} else {
				return string(d.ID)
			}
		}
		return nil, nil, fmt.Errorf("depository not found origDep=%q recDep=%q", p(origDep), p(recDep))
	}
	return origDep, recDep, nil
}

// updateDepositoryFromReturnCode will inspect the ach.ReturnCode and optionally update either the originating or receiving Depository.
// Updates are performed in cases like: death, account closure, authorization revoked, etc as specified in NACHA return codes.
//
// This function should never mark a Depository as Verified.
func updateDepositoryFromReturnCode(logger log.Logger, code *ach.ReturnCode, origDep *internal.Depository, destDep *internal.Depository, depRepo internal.DepositoryRepository) error {
	switch code.Code {
	case "R02", "R07", "R10": // "Account Closed", "Authorization Revoked by Customer", "Customer Advises Not Authorized"
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s for returnCode=%s", destDep.ID, code.Code))
		return depRepo.UpdateDepositoryStatus(destDep.ID, internal.DepositoryRejected)

	case "R05": // Improper Debit to Consumer Account
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s for returnCode=%s", destDep.ID, code.Code))
		return depRepo.UpdateDepositoryStatus(destDep.ID, internal.DepositoryRejected)

	case "R14", "R15": // "Representative payee deceased or unable to continue in that capacity", "Beneficiary or bank account holder"
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s and depository=%s for returnCode=%s", origDep.ID, destDep.ID, code.Code))
		if err := depRepo.UpdateDepositoryStatus(origDep.ID, internal.DepositoryRejected); err != nil {
			return err
		}
		return depRepo.UpdateDepositoryStatus(destDep.ID, internal.DepositoryRejected)

	case "R16": // "Bank account frozen"
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s for returnCode=%s", destDep.ID, code.Code))
		return depRepo.UpdateDepositoryStatus(destDep.ID, internal.DepositoryRejected)

	case "R20": // "Non-payment bank account"
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s for returnCode=%s", destDep.ID, code.Code))
		return depRepo.UpdateDepositoryStatus(destDep.ID, internal.DepositoryRejected)
	}
	return fmt.Errorf("unhandled return code: %s", code.Code)
}

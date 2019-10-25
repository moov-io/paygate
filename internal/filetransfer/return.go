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

func (c *Controller) processReturnFiles(dir string, depRepo internal.DepositoryRepository, transferRepo internal.TransferRepository) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if (err != nil && err != filepath.SkipDir) || info.IsDir() {
			return nil // Ignore SkipDir and directories
		}

		file, err := parseACHFilepath(path)
		if err != nil {
			c.cfg.Logger.Log("processReturnFiles", fmt.Sprintf("problem parsing return file %s", path), "error", err)
			return nil
		}
		c.cfg.Logger.Log("processReturnFiles", fmt.Sprintf("processing return file %s from %s (%s)", info.Name(), file.Header.ImmediateOriginName, file.Header.ImmediateOrigin))

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
					c.cfg.Logger.Log("processReturnFiles", "empty Addenda99 (or ReturnCode)", "traceNumber", entries[j].TraceNumber)
					continue
				}
				if err := c.processReturnEntry(file.Header, file.ReturnEntries[i].GetHeader(), entries[j], depRepo, transferRepo); err != nil {
					c.cfg.Logger.Log("processReturnFiles", "error processing EntryDetail", "traceNumber", entries[j].TraceNumber, "error", err)
					continue
				}
			}
		}
		return nil
	})
}

func (c *Controller) processReturnEntry(fileHeader ach.FileHeader, header *ach.BatchHeader, entry *ach.EntryDetail, depRepo internal.DepositoryRepository, transferRepo internal.TransferRepository) error {
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
		c.cfg.Logger.Log("processReturnEntry", fmt.Sprintf("matched traceNumber=%s to transfer=%s with returnCode=%s", entry.TraceNumber, transfer.ID, returnCode), "requestID", requestID)

		// Grab the full Depository objects for our Transfer
		origDep, err := depRepo.GetUserDepository(transfer.OriginatorDepository, transfer.UserID)
		if err != nil {
			return fmt.Errorf("processTransferReturn: error finding originator depository=%s: %v", transfer.OriginatorDepository, err)
		}
		recDep, err := depRepo.GetUserDepository(transfer.ReceiverDepository, transfer.UserID)
		if err != nil {
			return fmt.Errorf("processTransferReturn: error finding receiver depository=%s: %v", transfer.ReceiverDepository, err)
		}
		c.cfg.Logger.Log("processReturnEntry", fmt.Sprintf("found deposiories for transfer=%s (originator=%s) (receiver=%s)", transfer.ID, origDep.ID, recDep.ID), "requestID", requestID)

		// Optionally update the Depositories for this Transfer if the return code justifies it
		if err := updateDepositoryFromReturnCode(c.cfg.Logger, returnCode, origDep, recDep, depRepo); err != nil {
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
	if dep == nil || err != nil {
		return fmt.Errorf("problem looking up Depository: %v", err)
	}
	microDeposit, err := depRepo.LookupMicroDepositFromReturn(dep.ID, amount)
	if microDeposit != nil {
		if err := c.processMicroDepositReturn(requestID, dep.UserID(), dep.ID, microDeposit, depRepo, returnCode); err != nil {
			return fmt.Errorf("processMicroDepositReturn: %v", err)
		}
		c.cfg.Logger.Log("processReturnEntry", fmt.Sprintf("matched micro-deposit to depository=%s with returnCode=%s", dep.ID, returnCode), "requestID", requestID)

		// Optionally update the Depository for this micro-deposit if the return code justifies it
		if err := updateDepositoryFromReturnCode(c.cfg.Logger, returnCode, dep, dep, depRepo); err != nil {
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

// updateDepositoryFromReturnCode will inspect the ach.ReturnCode and optionally update either the originating or receiving Depository.
// Updates are performed in cases like: death, account closure, authorization revoked, etc as specified in NACHA return codes.
//
// This function should never mark a Depository as Verified.
//
// You can find all the NACHA return codes in their guidelines PDF, but some websites also republish the list.
// See: https://docs.moderntreasury.com/reference#ach-return-reason-codes
func updateDepositoryFromReturnCode(logger log.Logger, code *ach.ReturnCode, origDep *internal.Depository, destDep *internal.Depository, depRepo internal.DepositoryRepository) error {
	switch code.Code {
	// The following codes mark the Receiver Depository as Rejected because of a reason similar to
	// authorization changing, incorrect account/routing numbers, or human interaction is required.
	case
		"R02", // Account Closed
		"R03", // No Account/Unable to Locate Account
		"R04", // Invalid Account Number Structure
		"R05", // Improper Debit to Consumer Account
		"R07", // Authorization Revoked by Customer
		"R10", // Customer Advises Not Authorized
		"R12", // Account Sold to Another DFI
		"R13", // Invalid ACH Routing Number
		"R16", // Account Frozen/Entry Returned per OFAC Instruction
		"R20", // Non-payment (or non-transaction) bank account
		"R28", // Routing Number Check Digit Error
		"R29", // Corporate Customer Advises Not Authorized
		"R30", // RDFI Not Participant in Check Truncation Program
		"R32", // RDFI Non-Settlement
		"R34", // Limited Participation DFI
		"R37", // Source Document Presented for Payment
		"R38", // Stop Payment on Source Document
		"R39": // Improper Source Document/Source Document Presented for Payment
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s for returnCode=%s", destDep.ID, code.Code))
		return depRepo.UpdateDepositoryStatus(destDep.ID, internal.DepositoryRejected)

	// The following codes do not impact a Depository, but are handled here for informational logs.
	// Many of these return codes likely signal there's a bug in paygate or moov's ACH library.
	case
		"R01", // Insufficient Funds
		"R06", // Returned per ODFI's Request
		"R08", // Payment Stopped
		"R09", // Uncollected Funds
		"R11", // Check Truncation Early Return
		"R17", // File Record Edit Criteria
		"R18", // Improper Effective Entry Date
		"R19", // Amount Field Error
		"R21", // Invalid Company Identification
		"R22", // Invalid Individual ID Number
		"R23", // Credit Entry Refused by Receiver
		"R24", // Duplicate Entry
		"R25", // Addenda Error
		"R26", // Mandatory Field Error
		"R27", // Trace Number Error
		"R31", // Permissible Return Entry (CCD and CTX Only)
		"R33", // Return of XCK Entry
		"R35", // Return of Improper Debit Entry
		"R36": // Return of Improper Credit Entry
		logger.Log("processReturnEntry", fmt.Sprintf("handled depository=%s returnCode=%s", destDep.ID, code.Code))
		return nil

	case "R14", "R15": // "Representative payee deceased or unable to continue in that capacity", "Beneficiary or bank account holder"
		logger.Log("processReturnEntry", fmt.Sprintf("rejecting depository=%s and depository=%s for returnCode=%s", origDep.ID, destDep.ID, code.Code))
		if err := depRepo.UpdateDepositoryStatus(origDep.ID, internal.DepositoryRejected); err != nil {
			return err
		}
		return depRepo.UpdateDepositoryStatus(destDep.ID, internal.DepositoryRejected)
	}
	return fmt.Errorf("unhandled return code: %s", code.Code)
}

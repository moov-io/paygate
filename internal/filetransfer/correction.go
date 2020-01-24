// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/pkg/id"
)

func (c *Controller) handleNOCFile(req *periodicFileOperationsRequest, file *ach.File, filename string, depRepo internal.DepositoryRepository, transferRepo internal.TransferRepository) error {
	for i := range file.NotificationOfChange {
		entries := file.NotificationOfChange[i].GetEntries()
		for j := range entries {
			if entries[j].Addenda98 == nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("nil Addenda98 in EntryDetail file=%s", filename),
					"traceNumber", entries[j].TraceNumber,
					"userID", req.userID, "requestID", req.requestID)
				continue
			}

			changeCode := entries[j].Addenda98.ChangeCodeField()
			if changeCode == nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("no ChangeCode found code=%s file=%s", entries[j].Addenda98.ChangeCode, filename),
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.userID, "requestID", req.requestID)
				break
			}

			dep, _ := depRepo.LookupDepositoryFromReturn(file.Header.ImmediateDestination, strings.TrimSpace(entries[j].DFIAccountNumber))
			if dep == nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("depository not found file=%s", filename),
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.userID, "requestID", req.requestID)
				break
			} else {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("matched depository=%s", dep.ID),
					"traceNumber", entries[j].TraceNumber,
					"userID", req.userID, "requestID", req.requestID)
			}

			batchHeader := file.NotificationOfChange[i].GetHeader()
			if err := c.rejectRelatedObjectsForChangeCode(changeCode, batchHeader, entries[j], dep, depRepo, transferRepo); err != nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("error updating related objects to depository=%s from NOC code=%s", dep.ID, changeCode.Code), "error", err,
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.userID, "requestID", req.requestID)
			}

			if err := c.updateDepositoryFromChangeCode(changeCode, entries[j], dep, depRepo); err != nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("error updating depository=%s from NOC code=%s", dep.ID, changeCode.Code), "error", err,
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.userID, "requestID", req.requestID)
			} else {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("updated depository=%s from NOC code=%s", dep.ID, changeCode.Code),
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.userID, "requestID", req.requestID)
			}
		}
	}
	return nil
}

func (c *Controller) rejectRelatedObjectsForChangeCode(code *ach.ChangeCode, header *ach.BatchHeader, ed *ach.EntryDetail, dep *internal.Depository, depRepo internal.DepositoryRepository, transferRepo internal.TransferRepository) error {
	// TODO(adam): Should we be updating Originator and/or Receiver objects?
	// TODO(adam): We should probably write an event about rejecting the Depository

	// If we aren't going to be updating Depository fields then Reject the Depository
	if !c.updateDepositoriesFromNOCs {
		if err := depRepo.UpdateDepositoryStatus(dep.ID, internal.DepositoryRejected); err != nil {
			return fmt.Errorf("depository error: %v", err)
		}
	}

	amount, err := internal.NewAmountFromInt("USD", ed.Amount)
	if err != nil {
		return fmt.Errorf("invalid amount: %v", ed.Amount)
	}
	effectiveEntryDate, err := header.LiftEffectiveEntryDate()
	if err != nil {
		return fmt.Errorf("invalid EffectiveEntryDate=%q: %v", header.EffectiveEntryDate, err)
	}

	// Mark the transfer as Reclaimed due to error
	transfer, err := transferRepo.LookupTransferFromReturn(header.StandardEntryClassCode, amount, ed.TraceNumber, effectiveEntryDate)
	if err != nil {
		return fmt.Errorf("problem finding transfer: %v", err)
	}
	if err := transferRepo.UpdateTransferStatus(transfer.ID, internal.TransferReclaimed); err != nil {
		return fmt.Errorf("problem updating transfer=%q: %v", transfer.ID, err)
	}

	return nil
}

func (c *Controller) updateDepositoryFromChangeCode(code *ach.ChangeCode, ed *ach.EntryDetail, dep *internal.Depository, depRepo internal.DepositoryRepository) error {
	if dep == nil {
		return errors.New("depository not found")
	}

	if !c.updateDepositoriesFromNOCs {
		return fmt.Errorf("skipping depository=%s update from NOC code=%s", dep.ID, code.Code)
	}

	cor := ed.Addenda98.ParseCorrectedData()
	if cor == nil {
		return errors.New("missing Addenda98 record")
	}

	// Fixup account numbers
	if code.Code == "C01" || code.Code == "C03" || code.Code == "C06" || code.Code == "C07" {
		if num, err := c.keeper.EncryptString(cor.AccountNumber); err != nil {
			return err
		} else {
			dep.EncryptedAccountNumber = num
		}
	}
	// Fixup routing number
	if code.Code == "C02" || code.Code == "C03" || code.Code == "C07" {
		dep.RoutingNumber = cor.RoutingNumber
	}
	// Upsert the Depository after our changes
	if err := depRepo.UpsertUserDepository(id.User(dep.UserID()), dep); err != nil {
		return err
	}

	// Fixup individual name
	if code.Code == "C04" {
		return errors.New("skipping receiver individual name update")
	}
	// fixup identification
	if code.Code == "C09" {
		return errors.New("skipping originator identification name update")
	}

	// Checkout
	switch code.Code {
	case "C08": // Incorrect Receiving DFI Identification (IAT Only) // unsupported
		c.logger.Log("changeCode", fmt.Sprintf("rejecting depository=%s for IAT changeCode=%s", dep.ID, code.Code))
		return depRepo.UpdateDepositoryStatus(dep.ID, internal.DepositoryRejected)

	case "C05", "C06", "C07":
		err := depRepo.UpdateDepositoryStatus(dep.ID, internal.DepositoryRejected)
		return fmt.Errorf("rejecting originalTrace=%s after new transactionCode=%d was returned: %v", ed.Addenda98.OriginalTrace, cor.TransactionCode, err)

	// Internal errors
	case "C13", "C14": // Addenda Format Error, Incorrect SEC Code for Outbound International Payment
		c.logger.Log("changeCode", fmt.Sprintf("rejecting depository=%s due to internal error changeCode=%s", dep.ID, code.Code))
		return fmt.Errorf("unrecoverable problem with Addenda98 (code=%s)", code.Code)
	}

	return nil
}

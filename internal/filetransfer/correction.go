// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/filetransfer/admin"
	"github.com/moov-io/paygate/internal/model"
)

func (c *Controller) handleNOCFile(req *admin.Request, file *ach.File, filename string) error {
	for i := range file.NotificationOfChange {
		entries := file.NotificationOfChange[i].GetEntries()
		for j := range entries {
			if entries[j].Addenda98 == nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("nil Addenda98 in EntryDetail file=%s", filename),
					"traceNumber", entries[j].TraceNumber,
					"userID", req.UserID, "requestID", req.RequestID)
				continue
			}

			changeCode := entries[j].Addenda98.ChangeCodeField()
			if changeCode == nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("no ChangeCode found code=%s file=%s", entries[j].Addenda98.ChangeCode, filename),
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.UserID, "requestID", req.RequestID)
				break
			}

			dep, _ := c.depRepo.LookupDepositoryFromReturn(file.Header.ImmediateDestination, strings.TrimSpace(entries[j].DFIAccountNumber))
			if dep == nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("depository not found file=%s", filename),
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.UserID, "requestID", req.RequestID)
				break
			} else {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("matched depository=%s", dep.ID),
					"traceNumber", entries[j].TraceNumber,
					"userID", req.UserID, "requestID", req.RequestID)
			}

			batchHeader := file.NotificationOfChange[i].GetHeader()
			if err := c.rejectRelatedObjects(batchHeader, entries[j], dep); err != nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("error updating related objects to depository=%s from NOC code=%s", dep.ID, changeCode.Code), "error", err,
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.UserID, "requestID", req.RequestID)
			}

			if err := c.updateDepositoryFromChangeCode(changeCode, entries[j], dep); err != nil {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("error updating depository=%s from NOC code=%s", dep.ID, changeCode.Code), "error", err,
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.UserID, "requestID", req.RequestID)
			} else {
				c.logger.Log(
					"handleNOCFile", fmt.Sprintf("updated depository=%s from NOC code=%s", dep.ID, changeCode.Code),
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.UserID, "requestID", req.RequestID)
			}
		}
	}
	return nil
}

func (c *Controller) rejectRelatedObjects(header *ach.BatchHeader, ed *ach.EntryDetail, dep *model.Depository) error {
	// TODO(adam): Should we be updating Originator and/or Receiver objects?
	// TODO(adam): We should probably write an event about rejecting the Depository

	// If we aren't going to be updating Depository fields then Reject the Depository
	// as the fields being updated will keep the Depository verified.
	if !c.updateDepositoriesFromNOCs {
		if err := c.depRepo.UpdateDepositoryStatus(dep.ID, model.DepositoryRejected); err != nil {
			return fmt.Errorf("depository error: %v", err)
		}
	}

	amount, err := model.NewAmountFromInt("USD", ed.Amount)
	if err != nil {
		return fmt.Errorf("invalid amount: %v", ed.Amount)
	}
	effectiveEntryDate, err := header.LiftEffectiveEntryDate()
	if err != nil {
		return fmt.Errorf("invalid EffectiveEntryDate=%q: %v", header.EffectiveEntryDate, err)
	}

	// Mark the transfer as Reclaimed due to error
	transfer, err := c.transferRepo.LookupTransferFromReturn(header.StandardEntryClassCode, amount, ed.TraceNumber, effectiveEntryDate)
	if err != nil {
		return fmt.Errorf("problem finding transfer: %v", err)
	}
	if transfer == nil {
		return errors.New("transfer not found")
	}
	if err := c.transferRepo.UpdateTransferStatus(transfer.ID, model.TransferReclaimed); err != nil {
		return fmt.Errorf("problem updating transfer=%q: %v", transfer.ID, err)
	}

	return nil
}

func (c *Controller) updateDepositoryFromChangeCode(code *ach.ChangeCode, ed *ach.EntryDetail, dep *model.Depository) error {
	if dep == nil {
		return errors.New("depository not found")
	}

	// Skip any attempt to update fields if it's disabled.
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
	if err := c.depRepo.UpsertUserDepository(dep.UserID, dep); err != nil {
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
		return c.depRepo.UpdateDepositoryStatus(dep.ID, model.DepositoryRejected)

	case "C05", "C06", "C07":
		err := c.depRepo.UpdateDepositoryStatus(dep.ID, model.DepositoryRejected)
		return fmt.Errorf("rejecting originalTrace=%s after new transactionCode=%d was returned: %v", ed.Addenda98.OriginalTrace, cor.TransactionCode, err)

	// Internal errors
	case "C13", "C14": // Addenda Format Error, Incorrect SEC Code for Outbound International Payment
		c.logger.Log("changeCode", fmt.Sprintf("rejecting depository=%s due to internal error changeCode=%s", dep.ID, code.Code))
		return fmt.Errorf("unrecoverable problem with Addenda98 (code=%s)", code.Code)
	}

	return nil
}

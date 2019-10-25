// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal"

	"github.com/go-kit/kit/log"
)

func (c *Controller) handleNOCFile(req *periodicFileOperationsRequest, file *ach.File, filename string, depRepo internal.DepositoryRepository) error {
	for i := range file.NotificationOfChange {
		entries := file.NotificationOfChange[i].GetEntries()
		for j := range entries {
			if entries[j].Addenda98 == nil {
				c.cfg.Logger.Log(
					"handleNOCFile", fmt.Sprintf("nil Addenda98 in EntryDetail file=%s", filename),
					"traceNumber", entries[j].TraceNumber,
					"userID", req.userID, "requestID", req.requestID)
				continue
			}

			changeCode := entries[j].Addenda98.ChangeCodeField()
			if changeCode == nil {
				c.cfg.Logger.Log(
					"handleNOCFile", fmt.Sprintf("no ChangeCode found code=%s file=%s", entries[j].Addenda98.ChangeCode, filename),
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.userID, "requestID", req.requestID)
				break
			}

			dep, _ := depRepo.LookupDepositoryFromReturn(file.Header.ImmediateDestination, strings.TrimSpace(entries[j].DFIAccountNumber))
			if dep == nil {
				c.cfg.Logger.Log(
					"handleNOCFile", fmt.Sprintf("depository not found file=%s", filename),
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.userID, "requestID", req.requestID)
				break
			} else {
				c.cfg.Logger.Log(
					"handleNOCFile", fmt.Sprintf("matched depository=%s", dep.ID),
					"traceNumber", entries[j].TraceNumber,
					"userID", req.userID, "requestID", req.requestID)
			}

			if err := updateDepositoryFromChangeCode(c.cfg.Logger, changeCode, entries[j], dep, depRepo); err != nil {
				c.cfg.Logger.Log(
					"handleNOCFile", fmt.Sprintf("error updating depository=%s from NOC code=%s", dep.ID, changeCode.Code), "error", err,
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.userID, "requestID", req.requestID)
			} else {
				c.cfg.Logger.Log(
					"handleNOCFile", fmt.Sprintf("updated depository=%s from NOC code=%s", dep.ID, changeCode.Code),
					"traceNumber", entries[j].TraceNumber,
					"originalTrace", entries[j].Addenda98.OriginalTrace,
					"userID", req.userID, "requestID", req.requestID)
			}
		}
	}
	return nil
}

func updateDepositoryFromChangeCode(logger log.Logger, code *ach.ChangeCode, ed *ach.EntryDetail, dep *internal.Depository, depRepo internal.DepositoryRepository) error {
	if dep == nil {
		return errors.New("depository not found")
	}
	corrected := ed.Addenda98.CorrectedData
	switch code.Code {
	// Cases where we could probably update data automatically
	case
		"C01", // Incorrect DFI Account Number (first 17 of CorrectedData might have better value)
		"C02", // Incorrect Routing Number (first 9 of CorrectedData might have better value)
		"C03", // Incorrect Routing Number and Incorrect DFI Account Number (first 9 and 13th-29th) spaces in 10-12
		"C04", // Incorrect Individual Name / Receiving Company Name (first 22)
		"C06", // Incorrect DFI Account Number and Incorrect Transaction Code (first 17, then 21st and 22nd)
		"C07", // Incorrect Routing Number, Incorrect DFI Account Number, and Incorrect Tranaction Code. (first 9, 10th-26th, 27th-28th)
		"C09": // Incorrect Individual Identification Number/Incorrect Receiver Identification Number (first 22)
		// The Depository's account and/or routing number is invalid, so we probably didn't even find one.
		logger.Log("changeCode", fmt.Sprintf("rejecting depository=%s for changeCode=%s (corrected data: '%s')", dep.ID, code.Code, corrected))
		return depRepo.UpdateDepositoryStatus(dep.ID, internal.DepositoryRejected)

	// Unsupported cases (for now)
	case
		"C08": // Incorrect Receiving DFI Identification (IAT Only)
		logger.Log("changeCode", fmt.Sprintf("rejecting depository=%s for IAT changeCode=%s", dep.ID, code.Code))
		return depRepo.UpdateDepositoryStatus(dep.ID, internal.DepositoryRejected)

	// Internal errors
	case
		"C05", // Incorrect Transaction Code (first 2)
		"C13", // Addenda Format Error (unrecoverable)
		"C14": // Incorrect SEC Code for Outbound International Payment
		logger.Log("changeCode", fmt.Sprintf("rejecting depository=%s due to internal error changeCode=%s", dep.ID, code.Code))
		return fmt.Errorf("unrecoverable problem with Addenda98 (code=%s)", code.Code)

	default:
		return fmt.Errorf("unhandled change code %s", code.Code)
	}
}

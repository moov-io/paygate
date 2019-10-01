// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal"

	"github.com/go-kit/kit/log"
)

func (c *Controller) handleNOCFile(file *ach.File, depRepo internal.DepositoryRepository) error {
	for i := range file.NotificationOfChange {
		entries := file.NotificationOfChange[i].GetEntries()
		for j := range entries {
			if entries[j].Addenda98 == nil {
				c.logger.Log("handleNOCFile", fmt.Sprintf("nil Addenda98 in EntryDetail traceNumber=%s", entries[j].TraceNumber))
				continue
			}

			changeCode := entries[j].Addenda98.ChangeCodeField()
			dep := &internal.Depository{ID: "TODO(adam)"} // TODO(adam): dep can be nil (as values might be wrong)

			if err := updateDepositoryFromChangeCode(c.logger, changeCode, entries[j], dep, depRepo); err != nil {
				c.logger.Log("handleNOCFile", fmt.Sprintf("error updating depository=%s from NOC code=%s", dep.ID, changeCode.Code))
			} else {
				c.logger.Log("handleNOCFile", fmt.Sprintf("updated depository=%s from NOC code=%s", dep.ID, changeCode.Code))
			}
		}
	}
	return nil
}

func updateDepositoryFromChangeCode(logger log.Logger, code *ach.ChangeCode, ed *ach.EntryDetail, dep *internal.Depository, depRepo internal.DepositoryRepository) error {
	corrected := ed.Addenda98.CorrectedData
	switch code.Code {
	case
		"C01", // Incorrect DFI Account Number (first 17 of CorrectedData might have better value)
		"C02", // Incorrect Routing Number (first 9 of CorrectedData might have better value)
		"C03": // Incorrect Routing Number and Incorrect DFI Account Number (first 9 and 13th-29th) spaces in 10-12
		// The Depository's account and/or routing number is invalid, so we probably didn't even find one.
		logger.Log("changeCode", fmt.Sprintf("rejecting depository=%s for changeCode=%s", dep.ID, code.Code))
		return depRepo.UpdateDepositoryStatus(dep.ID, internal.DepositoryRejected)

	case "C04": // Incorrect Individual Name / Receiving Company Name (first 22)
		logger.Log("changeCode", fmt.Sprintf("bad: ed.IndividualName=%s | good: ed.IndividualName=%s", ed.IndividualName, corrected))

	case "C05": // Incorrect Transaction Code (first 2)
		logger.Log("changeCode", fmt.Sprintf("bad: ed.TransactionCode=%d | good: ed.TransactionCode=%s", ed.TransactionCode, corrected))

	case "C06": // Incorrect DFI Account Number and Incorrect Transaction Code (first 17, then 21st and 22nd)
		logger.Log("changeCode", fmt.Sprintf("C06: TODO(adam)"))

	case "C07": // Incorrect Routing Number, Incorrect DFI Account Number, and Incorrect Tranaction Code. (first 9, 10th-26th, 27th-28th)
		logger.Log("changeCode", fmt.Sprintf("C07: TODO(adam)"))

	case "C08": // Incorrect Receiving DFI Identification (IAT Only)
		return fmt.Errorf("unimplemented change code %s", code.Code)

	case "C09": // Incorrect Individual Identification Number/Incorrect Receiver Identification Number (first 22)
		logger.Log("changeCode", fmt.Sprintf("bad: ed.IdentificationNumber=%s | good: ed.IdentificationNumber=%s", ed.IdentificationNumber, corrected))

	case "C13": // Addenda Format Error (unrecoverable)
		return fmt.Errorf("unrecoverable problem with Addenda (code=%s)", code.Code)

	case "C14": // Incorrect SEC Code for Outbound International Payment
		return fmt.Errorf("unimplemented change code %s", code.Code)

	default:
		return fmt.Errorf("unhandled change code %s", code.Code)
	}
	return nil
}

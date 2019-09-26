// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal"
)

func (c *Controller) processTransferReturn(requestID string, transfer *internal.Transfer, transferRepo internal.TransferRepository, returnCode *ach.ReturnCode) error {
	// Set the ReturnCode and update the transfer's status
	if err := transferRepo.SetReturnCode(transfer.ID, returnCode.Code); err != nil {
		return fmt.Errorf("problem updating ReturnCode transfer=%q: %v", transfer.ID, err)
	}
	if err := transferRepo.UpdateTransferStatus(transfer.ID, internal.TransferReclaimed); err != nil {
		return fmt.Errorf("problem updating transfer=%q: %v", transfer.ID, err)
	}

	// Reverse the transaction against Accounts
	if c.accountsClient != nil && transfer.TransactionID != "" {
		if err := c.accountsClient.ReverseTransaction(requestID, transfer.UserID, transfer.TransactionID); err != nil {
			return fmt.Errorf("problem with accounts ReverseTransaction: %v", err)
		}
	} else {
		if transfer.TransactionID == "" {
			c.logger.Log("processTransferReturn", fmt.Sprintf("transfer=%s has no transactionID", transfer.ID), "requestID", requestID, "userID", transfer.UserID)
		}
	}

	return nil
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
		if deps[k].Status != internal.DepositoryVerified {
			continue // We only allow Verified Depositories
		}
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

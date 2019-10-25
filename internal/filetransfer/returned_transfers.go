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
			c.cfg.Logger.Log("processTransferReturn", fmt.Sprintf("transfer=%s has no transactionID", transfer.ID), "requestID", requestID, "userID", transfer.UserID)
		}
	}

	return nil
}

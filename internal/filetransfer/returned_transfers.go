// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/transfers"
	"github.com/moov-io/paygate/pkg/id"
)

func (c *Controller) processTransferReturn(requestID string, transfer *model.Transfer, transferRepo transfers.Repository, returnCode *ach.ReturnCode) error {
	// Set the ReturnCode and update the transfer's status
	if err := transferRepo.SetReturnCode(transfer.ID, returnCode.Code); err != nil {
		return fmt.Errorf("problem updating ReturnCode transfer=%q: %v", transfer.ID, err)
	}
	if err := transferRepo.UpdateTransferStatus(transfer.ID, model.TransferReclaimed); err != nil {
		return fmt.Errorf("problem updating transfer=%q: %v", transfer.ID, err)
	}

	// Reverse the transaction against Accounts
	if c.accountsClient != nil && transfer.TransactionID != "" {
		if err := c.accountsClient.ReverseTransaction(requestID, id.User(transfer.UserID), transfer.TransactionID); err != nil {
			return fmt.Errorf("problem with accounts ReverseTransaction: %v", err)
		}
	} else {
		if transfer.TransactionID == "" {
			c.logger.Log("processTransferReturn", fmt.Sprintf("transfer=%s has no transactionID", transfer.ID), "requestID", requestID, "userID", transfer.UserID)
		}
	}

	return nil
}

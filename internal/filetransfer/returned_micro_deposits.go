// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal"
)

func (c *Controller) processMicroDepositReturn(requestID, userID string, depID internal.DepositoryID, md *internal.MicroDeposit, depRepo internal.DepositoryRepository, code *ach.ReturnCode) error {
	if err := depRepo.SetReturnCode(depID, md.Amount, code.Code); err != nil {
		return fmt.Errorf("problem setting micro-deposit code=%s: %v", code.Code, err)
	}

	// Reverse micro-deposit transaction
	if c.accountsClient != nil && md.TransactionID != "" {
		if err := c.accountsClient.ReverseTransaction(requestID, userID, md.TransactionID); err != nil {
			return fmt.Errorf("problem reversing micro-deposit transaction=%s: %v", md.TransactionID, err)
		}
	} else {
		if md.TransactionID == "" {
			c.cfg.Logger.Log("processMicroDepositReturn", fmt.Sprintf("micro-deposit for depository=%s has no transaction", depID), "requestID", requestID, "userID", userID)
		}
	}

	return nil
}

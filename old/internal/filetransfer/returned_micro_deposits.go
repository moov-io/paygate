// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/depository/verification/microdeposit"
	"github.com/moov-io/paygate/pkg/id"
)

func (c *Controller) processMicroDepositReturn(requestID string, userID id.User, depID id.Depository, mc *microdeposit.Credit, code *ach.ReturnCode) error {
	if err := c.microDepositRepo.SetReturnCode(depID, mc.Amount, code.Code); err != nil {
		return fmt.Errorf("problem setting micro-deposit code=%s: %v", code.Code, err)
	}

	// Reverse micro-deposit transaction
	if c.accountsClient != nil && mc.TransactionID != "" {
		if err := c.accountsClient.ReverseTransaction(requestID, userID, mc.TransactionID); err != nil {
			return fmt.Errorf("problem reversing micro-deposit transaction=%s: %v", mc.TransactionID, err)
		}
	} else {
		if mc.TransactionID == "" {
			c.logger.Log("processMicroDepositReturn", fmt.Sprintf("micro-deposit for depository=%s has no transaction", depID), "requestID", requestID, "userID", userID)
		}
	}

	return nil
}

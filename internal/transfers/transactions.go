// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"errors"
	"fmt"

	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

// postAccountTransaction will lookup the Accounts for Depositories involved in a transfer and post the
// transaction against them in order to confirm, when possible, sufficient funds and other checks.
func (c *TransferRouter) postAccountTransaction(userID id.User, origDep *model.Depository, recDep *model.Depository, amount model.Amount, transferType model.TransferType, requestID string) (*accounts.Transaction, error) {
	if c.accountsClient == nil {
		return nil, errors.New("Accounts enabled but nil client")
	}

	// Let's lookup both accounts. Either account can be "external" (meaning of a RoutingNumber Accounts doesn't control).
	// When the routing numbers don't match we can't do much verify the remote account as we likely don't have Account-level access.
	//
	// TODO(adam): What about an FI that handles multiple routing numbers? Should Accounts expose which routing numbers it currently supports?
	receiverAccount, err := c.accountsClient.SearchAccounts(requestID, userID, recDep)
	if err != nil || receiverAccount == nil {
		return nil, fmt.Errorf("error reading account user=%s receiver depository=%s: %v", userID, recDep.ID, err)
	}
	origAccount, err := c.accountsClient.SearchAccounts(requestID, userID, origDep)
	if err != nil || origAccount == nil {
		return nil, fmt.Errorf("error reading account user=%s originator depository=%s: %v", userID, origDep.ID, err)
	}
	// Submit the transactions to Accounts (only after can we go ahead and save off the Transfer)
	transaction, err := c.accountsClient.PostTransaction(requestID, userID, createTransactionLines(origAccount, receiverAccount, amount, transferType))
	if err != nil {
		return nil, fmt.Errorf("error creating transaction for transfer user=%s: %v", userID, err)
	}
	c.logger.Log("transfers", fmt.Sprintf("created transaction=%s for user=%s amount=%s", transaction.ID, userID, amount.String()))
	return transaction, nil
}

func createTransactionLines(orig *accounts.Account, rec *accounts.Account, amount model.Amount, transferType model.TransferType) []accounts.TransactionLine {
	lines := []accounts.TransactionLine{
		{AccountID: orig.ID, Amount: int32(amount.Int())}, // originator
		{AccountID: rec.ID, Amount: int32(amount.Int())},  // receiver
	}
	if transferType == model.PullTransfer {
		lines[0].Purpose, lines[1].Purpose = "ACHCredit", "ACHDebit"
	} else {
		lines[0].Purpose, lines[1].Purpose = "ACHDebit", "ACHCredit"
	}
	return lines
}

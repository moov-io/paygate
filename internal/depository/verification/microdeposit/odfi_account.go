// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/internal/util"
	"github.com/moov-io/paygate/pkg/id"
)

// ODFIAccount represents the depository account micro-deposts are debited from
type ODFIAccount struct {
	accountNumber string
	routingNumber string
	accountType   model.AccountType

	client accounts.Client

	keeper *secrets.StringKeeper

	mu        sync.Mutex
	accountID string
}

func NewODFIAccount(accountsClient accounts.Client, accountNumber string, routingNumber string, accountType model.AccountType, keeper *secrets.StringKeeper) *ODFIAccount {
	return &ODFIAccount{
		client:        accountsClient,
		accountNumber: accountNumber,
		routingNumber: routingNumber,
		accountType:   accountType,
		keeper:        keeper,
	}
}

func (a *ODFIAccount) getID(requestID string, userID id.User) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Note: In environments where the ODFI accountID changes paygate won't notice the change
	// and so all micro-deposit calls will fail (or post to the wrong account).

	if a.accountID != "" {
		return a.accountID, nil
	}
	if a.client == nil {
		return "", errors.New("ODFIAccount: nil AccountsClient")
	}

	// Otherwise, make our Accounts HTTP call and grab the ID
	dep := &model.Depository{
		RoutingNumber: a.routingNumber,
		Type:          a.accountType,
	}
	dep.Keeper = a.keeper
	dep.ReplaceAccountNumber(a.accountNumber)

	acct, err := a.client.SearchAccounts(requestID, userID, dep)
	if err != nil || (acct == nil || acct.ID == "") {
		return "", fmt.Errorf("ODFIAccount: problem getting accountID: %v", err)
	}
	a.accountID = acct.ID // record account ID for calls later on
	return a.accountID, nil
}

func (a *ODFIAccount) Metadata() (*model.Originator, *model.Depository) {
	orig := &model.Originator{
		ID:                "odfi", // TODO(adam): make this NOT querable via db.
		DefaultDepository: id.Depository("odfi"),
		Identification:    util.Or(os.Getenv("ODFI_IDENTIFICATION"), "001"),
		Metadata:          "Moov - paygate micro-deposits",
	}
	num, err := a.keeper.EncryptString(a.accountNumber)
	if err != nil {
		return nil, nil
	}
	dep := &model.Depository{
		ID:                     id.Depository("odfi"),
		BankName:               util.Or(os.Getenv("ODFI_BANK_NAME"), "Moov, Inc"),
		Holder:                 util.Or(os.Getenv("ODFI_HOLDER"), "Moov, Inc"),
		HolderType:             model.Individual,
		Type:                   a.accountType,
		RoutingNumber:          a.routingNumber,
		EncryptedAccountNumber: num,
		Status:                 model.DepositoryVerified,
		Keeper:                 a.keeper,
	}
	return orig, dep
}

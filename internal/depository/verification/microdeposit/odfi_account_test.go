// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
	"errors"
	"testing"

	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
)

func makeTestODFIAccount() *ODFIAccount {
	// set routingNumber as ODFIIdentification in PPD batches (used in tests)
	account := NewODFIAccount(nil, "", "121042882", model.Checking, nil)
	account.accountID = "odfi-account"
	return account
}

func TestODFIAccount(t *testing.T) {
	keeper := secrets.TestStringKeeper(t)
	accountsClient := &accounts.MockClient{}

	num, _ := keeper.EncryptString("1234")
	odfi := &ODFIAccount{
		client:        accountsClient,
		accountNumber: num,
		routingNumber: "",
		accountType:   model.Savings,
		accountID:     "accountID",
		keeper:        keeper,
	}

	orig, dep := odfi.Metadata()
	if orig == nil || dep == nil {
		t.Fatalf("\norig=%#v\ndep=%#v", orig, dep)
	}
	if orig.ID != "odfi" {
		t.Errorf("originator: %#v", orig)
	}
	if string(dep.ID) != "odfi" {
		t.Errorf("depository: %#v", dep)
	}

	if accountID, err := odfi.getID("", "userID"); accountID != "accountID" || err != nil {
		t.Errorf("accountID=%s error=%v", accountID, err)
	}
	odfi.accountID = "" // unset so we make the AccountsClient call
	accountsClient.Accounts = []accounts.Account{
		{
			ID: "accountID2",
		},
	}
	if accountID, err := odfi.getID("", "userID"); accountID != "accountID2" || err != nil {
		t.Errorf("accountID=%s error=%v", accountID, err)
	}
	if odfi.accountID != "accountID2" {
		t.Errorf("odfi.accountID=%s", odfi.accountID)
	}

	// error on AccountsClient call
	odfi.accountID = ""
	accountsClient.Err = errors.New("bad")
	if accountID, err := odfi.getID("", "userID"); accountID != "" || err == nil {
		t.Errorf("expected error accountID=%s", accountID)
	}

	// on nil AccountsClient expect an error
	odfi.client = nil
	if accountID, err := odfi.getID("", "userID"); accountID != "" || err == nil {
		t.Errorf("expcted error accountID=%s", accountID)
	}
}

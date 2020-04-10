// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

func TestTransfers__createTransactionLines(t *testing.T) {
	orig := &accounts.Account{ID: base.ID()}
	rec := &accounts.Account{ID: base.ID()}
	amt, _ := model.NewAmount("USD", "12.53")

	lines := createTransactionLines(orig, rec, *amt, model.PushTransfer)
	if len(lines) != 2 {
		t.Errorf("got %d lines: %v", len(lines), lines)
	}

	// First transactionLine
	if lines[0].AccountID != orig.ID {
		t.Errorf("lines[0].AccountID=%s", lines[0].AccountID)
	}
	if !strings.EqualFold(lines[0].Purpose, "ACHDebit") {
		t.Errorf("lines[0].Purpose=%s", lines[0].Purpose)
	}
	if lines[0].Amount != 1253 {
		t.Errorf("lines[0].Amount=%d", lines[0].Amount)
	}

	// Second transactionLine
	if lines[1].AccountID != rec.ID {
		t.Errorf("lines[1].AccountID=%s", lines[1].AccountID)
	}
	if !strings.EqualFold(lines[1].Purpose, "ACHCredit") {
		t.Errorf("lines[1].Purpose=%s", lines[1].Purpose)
	}
	if lines[1].Amount != 1253 {
		t.Errorf("lines[1].Amount=%d", lines[1].Amount)
	}

	// flip the TransferType
	lines = createTransactionLines(orig, rec, *amt, model.PullTransfer)
	if !strings.EqualFold(lines[0].Purpose, "ACHCredit") {
		t.Errorf("lines[0].Purpose=%s", lines[0].Purpose)
	}
	if !strings.EqualFold(lines[1].Purpose, "ACHDebit") {
		t.Errorf("lines[1].Purpose=%s", lines[1].Purpose)
	}
}

func TestTransfers__postAccountTransaction(t *testing.T) {
	transferRepo := &MockRepository{}
	xferRouter := setupTestRouter(t, transferRepo)

	if a, ok := xferRouter.accountsClient.(*accounts.MockClient); ok {
		a.Accounts = []accounts.Account{
			{
				ID: base.ID(), // Just a stub, the fields aren't checked in this test
			},
		}
		a.Transaction = &accounts.Transaction{ID: base.ID()}
	} else {
		t.Fatalf("unknown accounts.Client: %T", xferRouter.accountsClient)
	}

	amt, _ := model.NewAmount("USD", "63.21")
	origDep := &model.Depository{
		EncryptedAccountNumber: "214124124",
		RoutingNumber:          "1215125151",
		Type:                   model.Checking,
	}
	recDep := &model.Depository{
		EncryptedAccountNumber: "212142",
		RoutingNumber:          "1215125151",
		Type:                   model.Savings,
	}

	userID, requestID := id.User(base.ID()), base.ID()
	tx, err := xferRouter.postAccountTransaction(userID, origDep, recDep, *amt, model.PullTransfer, requestID)
	if err != nil {
		t.Fatal(err)
	}
	if tx == nil {
		t.Errorf("nil accounts.Transaction")
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

func TestTransfers_transferCursor(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	keeper := secrets.TestStringKeeper(t)

	depRepo := depository.NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)
	transferRepo := &SQLRepo{db.DB, log.NewNopLogger()}

	userID := id.User(base.ID())
	amt := func(number string) model.Amount {
		amt, _ := model.NewAmount("USD", number)
		return *amt
	}

	dep := &model.Depository{
		ID:                     id.Depository(base.ID()),
		BankName:               "bank name",
		Holder:                 "holder",
		HolderType:             model.Individual,
		Type:                   model.Checking,
		RoutingNumber:          "123",
		EncryptedAccountNumber: "151",
		Status:                 model.DepositoryUnverified,
		Created:                base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	// Write transfers into database
	//
	// With the batch size low enough if more transfers are inserted within 1ms? than the batch size
	// the cursor will get stuck in an infinite loop. So we're inserting them at different times.
	//
	// TODO(adam): Will this become an issue?
	requests := []*transferRequest{
		{
			Type:                   model.PushTransfer,
			Amount:                 amt("12.12"),
			Originator:             model.OriginatorID("originator1"),
			OriginatorDepository:   dep.ID, // OriginatorDepository is read from a depositoryRepository
			Receiver:               model.ReceiverID("receiver1"),
			ReceiverDepository:     id.Depository("receiver1"),
			Description:            "money1",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file1",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}
	requests = []*transferRequest{
		{
			Type:                   model.PullTransfer,
			Amount:                 amt("13.13"),
			Originator:             model.OriginatorID("originator2"),
			OriginatorDepository:   dep.ID,
			Receiver:               model.ReceiverID("receiver2"),
			ReceiverDepository:     id.Depository("receiver2"),
			Description:            "money2",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file2",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}
	requests = []*transferRequest{
		{
			Type:                   model.PushTransfer,
			Amount:                 amt("14.14"),
			Originator:             model.OriginatorID("originator3"),
			OriginatorDepository:   dep.ID,
			Receiver:               model.ReceiverID("receiver3"),
			ReceiverDepository:     id.Depository("receiver3"),
			Description:            "money3",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file3",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}

	// Now verify the cursor pulls those transfers out
	cur := transferRepo.GetCursor(2, depRepo) // batch size
	firstBatch, err := cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(firstBatch) != 2 {
		for i := range firstBatch {
			t.Errorf("firstBatch[%d]=%#v", i, firstBatch[i])
		}
	}
	secondBatch, err := cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(secondBatch) != 1 {
		for i := range secondBatch {
			t.Errorf("secondBatch[%d]=%#v", i, secondBatch[i])
		}
	}
}

func TestTransfers_MarkTransferAsMerged(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	keeper := secrets.TestStringKeeper(t)
	depRepo := depository.NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)
	transferRepo := &SQLRepo{db.DB, log.NewNopLogger()}

	userID := id.User(base.ID())
	amt := func(number string) model.Amount {
		amt, _ := model.NewAmount("USD", number)
		return *amt
	}

	dep := &model.Depository{
		ID:                     id.Depository(base.ID()),
		BankName:               "bank name",
		Holder:                 "holder",
		HolderType:             model.Individual,
		Type:                   model.Checking,
		RoutingNumber:          "123",
		EncryptedAccountNumber: "151",
		Status:                 model.DepositoryVerified,
		Created:                base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	// Write transfers into database
	//
	// With the batch size low enough if more transfers are inserted within 1ms? than the batch size
	// the cursor will get stuck in an infinite loop. So we're inserting them at different times.
	//
	// TODO(adam): Will this become an issue?
	requests := []*transferRequest{
		{
			Type:                   model.PushTransfer,
			Amount:                 amt("12.12"),
			Originator:             model.OriginatorID("originator1"),
			OriginatorDepository:   id.Depository("originator1"),
			Receiver:               model.ReceiverID("receiver1"),
			ReceiverDepository:     dep.ID, // ReceiverDepository is read from a depositoryRepository
			Description:            "money1",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file1",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}

	// Now verify the cursor pulls those transfers out
	cur := transferRepo.GetCursor(2, depRepo) // batch size
	firstBatch, err := cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(firstBatch) != 1 {
		for i := range firstBatch {
			t.Errorf("firstBatch[%d]=%#v", i, firstBatch[i])
		}
		t.Fatalf("firstBatch: %#v", firstBatch)
	}

	// mark our transfer as merged, so we don't see it (in a new transferCursor we create)
	if err := transferRepo.MarkTransferAsMerged(firstBatch[0].ID, "merged-file.ach", "traceNumber"); err != nil {
		t.Fatal(err)
	}

	// re-create our transferCursor and see the transfer ignored
	// plus add a second transfer and ensure we get that
	requests = []*transferRequest{
		{
			Type:                   model.PullTransfer,
			Amount:                 amt("13.13"),
			Originator:             model.OriginatorID("originator2"),
			OriginatorDepository:   id.Depository("originator2"),
			Receiver:               model.ReceiverID("receiver2"),
			ReceiverDepository:     dep.ID,
			Description:            "money2",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file2",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}
	cur = transferRepo.GetCursor(2, depRepo) // batch size
	firstBatch, err = cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(firstBatch) != 1 {
		for i := range firstBatch {
			t.Errorf("firstBatch[%d].ID=%v amount=%v", i, firstBatch[i].ID, firstBatch[i].Amount.String())
		}
		t.Fatalf("firstBatch: %#v", firstBatch)
	}
	if firstBatch[0].Amount.String() != "USD 13.13" {
		t.Errorf("got %v", firstBatch[0].Amount.String())
	}
}

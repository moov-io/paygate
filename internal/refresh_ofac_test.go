// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"errors"
	"testing"
	"time"

	"github.com/moov-io/base"
	moovcustomers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
)

func TestOFACRefresh__oldEnough(t *testing.T) {
	if searchIsOldEnough(time.Now(), 1*time.Minute) {
		t.Error("now isn't 1 minute old")
	}

	if !searchIsOldEnough(time.Now().Add(-1*time.Minute), 10*time.Second) {
		t.Error("1 minute ago is older than 10s")
	}
}

func TestOFACRefresh(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	client := &customers.TestClient{Err: errors.New("bad error")}

	ref := NewRefresher(log.NewNopLogger(), client, db.DB)
	go func() {
		if err := ref.Start(1 * time.Millisecond); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(10 * time.Millisecond)
	ref.Close()
}

func TestOFACRefresh__refreshSearchIfNeeded(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	client := &customers.TestClient{
		Customer: &moovcustomers.Customer{
			ID:     base.ID(),
			Status: "ofac",
		},
		Result: &moovcustomers.OfacSearch{
			EntityId: "1512",
			SdnName:  "jane smith",
		},
		Err: errors.New("bad error"),
	}

	r := NewRefresher(log.NewNopLogger(), client, db.DB)
	ref, ok := r.(*periodicRefresher)
	if !ok {
		t.Fatalf("got %T", r)
	}

	err := ref.refreshSearchIfNeeded(customers.Cust{
		ID: base.ID(),
	}, "")
	if err == nil {
		t.Error("expected error")
	}

	client.Err = nil
	err = ref.refreshSearchIfNeeded(customers.Cust{
		ID:        base.ID(),
		CreatedAt: time.Now().Add(-1 * 30 * 24 * time.Hour),
	}, "")
	if err != nil {
		t.Error(err)
	}
}

func TestOFACRefresh__rejectRelatedCustomerObjects(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	userID := base.ID()

	depRepo := &SQLDepositoryRepo{db.DB, log.NewNopLogger()}
	receiverRepo := &SQLReceiverRepo{db.DB, log.NewNopLogger()}

	depID := base.ID()
	err := depRepo.UpsertUserDepository(userID, &Depository{
		ID:            DepositoryID(depID),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "121421212",
		AccountNumber: "1321",
		Status:        DepositoryUnverified,
		Metadata:      "metadata",
		Created:       base.NewTime(time.Now()),
		Updated:       base.NewTime(time.Now()),
	})
	if err != nil {
		t.Fatal(err)
	}

	customerID := base.ID()
	client := &customers.TestClient{
		Customer: &moovcustomers.Customer{
			ID:     customerID,
			Status: "Rejected",
		},
	}
	cust := customers.Cust{
		ID:                   customerID,
		OriginatorID:         base.ID(),
		OriginatorDepository: depID,
	}
	if err := rejectRelatedCustomerObjects(client, cust, "", depRepo, receiverRepo); err != nil {
		t.Fatal(err)
	}

	dep, err := depRepo.GetUserDepository(DepositoryID(depID), userID)
	if err != nil {
		t.Fatal(err)
	}
	if dep.Status != DepositoryRejected {
		t.Errorf("dep.Status=%v", dep.Status)
	}

	// now try with a receiver
	receiverID := base.ID()

	cust.OriginatorID = ""
	cust.ReceiverID = receiverID

	err = receiverRepo.upsertUserReceiver(userID, &Receiver{
		ID:                ReceiverID(receiverID),
		Email:             "test@moov.io",
		DefaultDepository: DepositoryID(base.ID()),
		Status:            ReceiverVerified,
		Metadata:          "extra data",
		Created:           base.NewTime(time.Now()),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := rejectRelatedCustomerObjects(client, cust, "", depRepo, receiverRepo); err != nil {
		t.Fatal(err)
	}

	receiver, err := receiverRepo.getUserReceiver(ReceiverID(receiverID), userID)
	if err != nil {
		t.Fatal(err)
	}
	if receiver.Status != ReceiverSuspended {
		t.Errorf("receiver.Status=%v", receiver.Status)
	}
}

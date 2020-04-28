// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

// import (
// 	"errors"
// 	"testing"

// 	"github.com/moov-io/base"
// 	"github.com/moov-io/paygate/internal/customers"
// 	"github.com/moov-io/paygate/internal/model"
// 	"github.com/moov-io/paygate/pkg/id"
// )

// func TestTransfers__verifyCustomerStatus(t *testing.T) {
// 	client := &customers.TestClient{
// 		Customer: &customers.Customer{
// 			ID:     base.ID(),
// 			Status: "ofac",
// 		},
// 	}
// 	orig := &model.Originator{
// 		CustomerID: base.ID(),
// 	}
// 	rec := &model.Receiver{
// 		CustomerID: base.ID(),
// 	}

// 	userID := id.User(base.ID())

// 	if err := verifyCustomerStatuses(orig, rec, client, base.ID(), userID); err != nil {
// 		t.Errorf("unexpected error: %v", err)
// 	}

// 	// set an unacceptable status
// 	client.Customer.Status = "reviewrequired"
// 	if err := verifyCustomerStatuses(orig, rec, client, base.ID(), userID); err == nil {
// 		t.Error("expected error")
// 	}

// 	// set an invalid status
// 	client.Customer.Status = "invalid"
// 	if err := verifyCustomerStatuses(orig, rec, client, base.ID(), userID); err == nil {
// 		t.Error("expected error")
// 	}

// 	// set an error and handle it
// 	client.Err = errors.New("bad error")
// 	if err := verifyCustomerStatuses(orig, rec, client, base.ID(), userID); err == nil {
// 		t.Error("expected error")
// 	}
// }

// func TestTransfers__verifyDisclaimersAreAccepted(t *testing.T) {
// 	orig := &model.Originator{CustomerID: base.ID()}
// 	rec := &model.Receiver{CustomerID: base.ID()}
// 	requestID, userID := base.ID(), id.User(base.ID())

// 	client := &customers.TestClient{}

// 	if err := verifyDisclaimersAreAccepted(orig, rec, client, requestID, userID); err != nil {
// 		t.Errorf("unexpected error: %v", err)
// 	}

// 	client.Err = errors.New("bad error")
// 	if err := verifyDisclaimersAreAccepted(orig, rec, client, requestID, userID); err == nil {
// 		t.Error("expected error")
// 	}
// }

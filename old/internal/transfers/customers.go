// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"fmt"

	moovcustomers "github.com/moov-io/customers"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

func verifyCustomerStatuses(orig *model.Originator, rec *model.Receiver, client customers.Client, requestID string, userID id.User) error {
	cust, err := client.Lookup(orig.CustomerID, requestID, userID)
	if err != nil {
		return fmt.Errorf("verifyCustomerStatuses: originator: %v", err)
	}
	status, err := moovcustomers.LiftStatus(cust.Status)
	if err != nil {
		return fmt.Errorf("verifyCustomerStatuses: lift originator: %v", err)
	}
	if !moovcustomers.ApprovedAt(*status, moovcustomers.OFAC) {
		return fmt.Errorf("verifyCustomerStatuses: customer=%s has unacceptable status=%s for Transfers", cust.ID, cust.Status)
	}

	cust, err = client.Lookup(rec.CustomerID, requestID, userID)
	if err != nil {
		return fmt.Errorf("verifyCustomerStatuses: receiver: %v", err)
	}
	status, err = moovcustomers.LiftStatus(cust.Status)
	if err != nil {
		return fmt.Errorf("verifyCustomerStatuses: lift receiver: %v", err)
	}
	if !moovcustomers.ApprovedAt(*status, moovcustomers.OFAC) {
		return fmt.Errorf("verifyCustomerStatuses: customer=%s has unacceptable status=%s for Transfers", cust.ID, cust.Status)
	}

	return nil
}

func verifyDisclaimersAreAccepted(orig *model.Originator, receiver *model.Receiver, client customers.Client, requestID string, userID id.User) error {
	if err := customers.HasAcceptedAllDisclaimers(client, orig.CustomerID, requestID, userID); err != nil {
		return fmt.Errorf("originator=%s: %v", orig.ID, err)
	}
	if err := customers.HasAcceptedAllDisclaimers(client, receiver.CustomerID, requestID, userID); err != nil {
		return fmt.Errorf("receiver=%s: %v", receiver.ID, err)
	}
	return nil
}

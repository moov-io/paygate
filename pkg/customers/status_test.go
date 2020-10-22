// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"testing"

	moovcustomers "github.com/moov-io/customers/pkg/client"
)

func TestAcceptableCustomerStatus(t *testing.T) {
	cust := &moovcustomers.Customer{}
	if err := AcceptableCustomerStatus(cust); err == nil {
		t.Error("expected error")
	}

	// failure
	cases := []moovcustomers.CustomerStatus{
		moovcustomers.CUSTOMERSTATUS_DECEASED,
		moovcustomers.CUSTOMERSTATUS_REJECTED,
		moovcustomers.CUSTOMERSTATUS_UNKNOWN,
	}
	for i := range cases {
		cust.Status = cases[i]
		if err := AcceptableCustomerStatus(cust); err == nil {
			t.Errorf("expected error with %s", cust.Status)
		}
	}

	// passing
	cases = []moovcustomers.CustomerStatus{
		moovcustomers.CUSTOMERSTATUS_RECEIVE_ONLY,
		moovcustomers.CUSTOMERSTATUS_VERIFIED,
	}
	for i := range cases {
		cust.Status = cases[i]
		if err := AcceptableCustomerStatus(cust); err != nil {
			t.Errorf("%s should have passed: %v", cust.Status, err)
		}
	}
}

func TestAcceptableAccountStatus(t *testing.T) {
	acct := &moovcustomers.Account{}
	if err := AcceptableAccountStatus(acct); err == nil {
		t.Error("expected error")
	}

	acct.Status = moovcustomers.ACCOUNTSTATUS_NONE
	if err := AcceptableAccountStatus(acct); err == nil {
		t.Errorf("expected error with %s", acct.Status)
	}

	acct.Status = moovcustomers.ACCOUNTSTATUS_VALIDATED
	if err := AcceptableAccountStatus(acct); err != nil {
		t.Errorf("%s should have passed: %v", acct.Status, err)
	}
}

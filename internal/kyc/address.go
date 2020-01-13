// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package kyc

import (
	moovcustomers "github.com/moov-io/customers/client"
)

// Address is an object for capturing US postal addresses
type Address struct {
	// Address1 is the first line of a postal address
	Address1 string `json:"address1,omitempty"`
	// Address2 is the second and optional line of a postal address
	Address2 string `json:"address2,omitempty"`
	// City is the name of a United States incorporated city
	City string `json:"city,omitempty"`
	// State is the two charcer code of a US state
	State string `json:"state,omitempty"`
	// PostalCode is a United States postal code
	PostalCode string `json:"postalCode,omitempty"`
}

func ConvertAddress(add *Address) []moovcustomers.CreateAddress {
	if add == nil {
		return nil
	}
	return []moovcustomers.CreateAddress{
		{
			Type:       "Primary",
			Address1:   add.Address1,
			Address2:   add.Address2,
			City:       add.City,
			State:      add.State,
			PostalCode: add.PostalCode,
			Country:    "US",
		},
	}
}

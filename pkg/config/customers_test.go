// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"testing"
)

func TestCustomers_validate(t *testing.T) {
	cfg := Customers{
		Accounts: Accounts{
			Decryptor: Decryptor{
				Symmetric: &Symmetric{
					KeyURI: "", // intentionally blank
				},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error")
	}
}

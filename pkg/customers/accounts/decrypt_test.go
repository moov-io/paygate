// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package accounts

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/moov-io/base"
	moovcustomers "github.com/moov-io/customers/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers"
)

var (
	plaintextAccountNumber = "123456"

	testSecretKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("1"), 32))
)

func TestDecryptor__Symmetric(t *testing.T) {
	cfg := config.Decryptor{
		Symmetric: &config.Symmetric{
			KeyURI: testSecretKey,
		},
	}
	client := &customers.MockClient{
		Transit: &moovcustomers.TransitAccountNumber{
			AccountNumber: "",
		},
	}

	decryptor, err := NewDecryptor(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	if d, ok := decryptor.(*symmetricDecryptor); ok {
		client.Transit.AccountNumber, err = d.keeper.EncryptString(plaintextAccountNumber)
	}
	if err != nil {
		t.Fatal(err)
	}

	num, err := decryptor.AccountNumber(base.ID(), base.ID())
	if err != nil {
		t.Fatal(err)
	}
	if num != plaintextAccountNumber {
		t.Errorf("got %q expected %q", num, plaintextAccountNumber)
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package accounts

type MockDecryptor struct {
	Number string
	Err    error
}

func (d *MockDecryptor) AccountNumber(customerID, accountID string) (string, error) {
	if d.Err != nil {
		return "", d.Err
	}
	return d.Number, nil
}

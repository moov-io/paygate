// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package accounts

import (
	"errors"
	"fmt"
	"time"

	"github.com/moov-io/customers/pkg/secrets"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers"
)

type Decryptor interface {
	AccountNumber(customerID, accountID string) (string, error)
}

func NewDecryptor(cfg config.Decryptor, client customers.Client) (Decryptor, error) {
	if cfg.Symmetric != nil {
		return createSymmetricDecryptor(cfg.Symmetric, client)
	}
	return nil, errors.New("unknown decryptor config")
}

type symmetricDecryptor struct {
	keeper    *secrets.StringKeeper
	customers customers.Client
}

func createSymmetricDecryptor(cfg *config.Symmetric, client customers.Client) (*symmetricDecryptor, error) {
	keeper, err := secrets.OpenLocal(cfg.KeyURI)
	if err != nil {
		return nil, err
	}
	return &symmetricDecryptor{
		keeper:    secrets.NewStringKeeper(keeper, 5*time.Second),
		customers: client,
	}, nil
}

func (dec *symmetricDecryptor) AccountNumber(customerID, accountID string) (string, error) {
	wrapper, err := dec.customers.DecryptAccount(customerID, accountID)
	if err != nil {
		return "", fmt.Errorf("problem reading full customerID=%s account=%s number error=%v", customerID, accountID, err)
	}
	num, err := dec.keeper.DecryptString(wrapper.AccountNumber)
	if err != nil {
		return "", fmt.Errorf("problem decrypting customerID=%s accountID=%s error=%v", customerID, accountID, err)
	}
	return num, nil
}

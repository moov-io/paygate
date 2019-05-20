// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	accounts "github.com/moov-io/accounts/client"
	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

type AccountsClient interface {
	Ping() error

	PostTransaction(requestId, userId string, lines []transactionLine) (*accounts.Transaction, error)
	SearchAccounts(requestId, userId string, dep *Depository) (*accounts.Account, error)
}

type moovAccountsClient struct {
	underlying *accounts.APIClient
	logger     log.Logger
}

func (c *moovAccountsClient) Ping() error {
	// create a context just for this so ping requests don't require the setup of one
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	resp, err := c.underlying.AccountsApi.Ping(ctx)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if resp == nil || err != nil {
		return fmt.Errorf("accounts ping failed: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("accounts ping got status: %s", resp.Status)
	}
	return err
}

type transactionLine struct {
	AccountId string
	Purpose   string
	Amount    int32
}

func (c *moovAccountsClient) PostTransaction(requestId, userId string, lines []transactionLine) (*accounts.Transaction, error) {
	if len(lines) == 0 {
		return nil, errors.New("accounts: no transactionLine's")
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	var accountsLines []accounts.TransactionLine
	for i := range lines {
		accountsLines = append(accountsLines, accounts.TransactionLine{
			AccountId: lines[i].AccountId,
			Purpose:   lines[i].Purpose,
			Amount:    float32(lines[i].Amount),
		})
	}
	req := accounts.CreateTransaction{accountsLines}
	tx, resp, err := c.underlying.AccountsApi.CreateTransaction(ctx, userId, req, &accounts.CreateTransactionOpts{
		XRequestId: optional.NewString(requestId),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return &tx, fmt.Errorf("accounts: PostTransaction: %v", err)
	}
	return &tx, nil
}

func (c *moovAccountsClient) SearchAccounts(requestId, userId string, dep *Depository) (*accounts.Account, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	c.logger.Log("accounts", fmt.Sprintf("searching for depository=%s account", dep.ID), "requestId", requestId)

	account, resp, err := c.underlying.AccountsApi.SearchAccounts(ctx, dep.AccountNumber, dep.RoutingNumber, string(dep.Type), userId, &accounts.SearchAccountsOpts{
		XRequestId: optional.NewString(requestId),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("accounts: SearchAccounts: depository=%s userId=%s: %v", dep.ID, userId, err)
	}
	return &account, nil
}

// createAccountsClient returns an AccountsClient used to make HTTP calls over to a Account instance.
// By default moov's localhost bind address will be used or the Kubernetes DNS name
// when called from inside a Kubernetes cluster.
//
// endpoint is a DNS record responsible for routing us to an Account instance.
// Example: http://accounts.apps.svc.cluster.local:8080
func createAccountsClient(logger log.Logger, endpoint string) AccountsClient {
	conf := accounts.NewConfiguration()
	conf.BasePath = "http://localhost" + bind.HTTP("accounts")

	if k8s.Inside() {
		conf.BasePath = "http://accounts.apps.svc.cluster.local:8080"
	}
	if endpoint != "" {
		conf.BasePath = endpoint
	}

	logger.Log("accounts", fmt.Sprintf("using %s for Accounts address", conf.BasePath))

	return &moovAccountsClient{
		underlying: accounts.NewAPIClient(conf),
		logger:     logger,
	}
}

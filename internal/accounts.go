// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	accounts "github.com/moov-io/accounts/client"
	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
)

type AccountsClient interface {
	Ping() error

	PostTransaction(requestID, userID string, lines []transactionLine) (*accounts.Transaction, error)
	SearchAccounts(requestID, userID string, dep *Depository) (*accounts.Account, error)
	ReverseTransaction(requestID, userID string, transactionID string) error
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
	AccountID string
	Purpose   string
	Amount    int32
}

func (c *moovAccountsClient) PostTransaction(requestID, userID string, lines []transactionLine) (*accounts.Transaction, error) {
	if len(lines) == 0 {
		return nil, errors.New("accounts: no transactionLine's")
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	var accountsLines []accounts.TransactionLine
	for i := range lines {
		accountsLines = append(accountsLines, accounts.TransactionLine{
			AccountID: lines[i].AccountID,
			Purpose:   lines[i].Purpose,
			Amount:    float32(lines[i].Amount),
		})
	}
	req := accounts.CreateTransaction{Lines: accountsLines}
	tx, resp, err := c.underlying.AccountsApi.CreateTransaction(ctx, userID, req, &accounts.CreateTransactionOpts{
		XRequestID: optional.NewString(requestID),
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return &tx, fmt.Errorf("accounts: PostTransaction: %v", err)
	}
	return &tx, nil
}

func (c *moovAccountsClient) SearchAccounts(requestID, userID string, dep *Depository) (*accounts.Account, error) {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	c.logger.Log("accounts", fmt.Sprintf("searching for depository=%s account", dep.ID), "requestID", requestID)

	opts := &accounts.SearchAccountsOpts{
		Number:        optional.NewString(dep.AccountNumber),
		RoutingNumber: optional.NewString(dep.RoutingNumber),
		Type_:         optional.NewString(string(dep.Type)),
		XRequestID:    optional.NewString(requestID),
	}
	accounts, resp, err := c.underlying.AccountsApi.SearchAccounts(ctx, userID, opts)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("accounts: SearchAccounts: depository=%s userID=%s: %v", dep.ID, userID, err)
	}
	if len(accounts) == 0 {
		return nil, nil // account not found
	}
	return &accounts[0], nil
}

func (c *moovAccountsClient) ReverseTransaction(requestID, userID string, transactionID string) error {
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()

	c.logger.Log("accounts", fmt.Sprintf("reversing transaction=%s", transactionID), "requestID", requestID)

	opts := &accounts.ReverseTransactionOpts{
		XRequestID: optional.NewString(requestID),
	}
	_, resp, err := c.underlying.AccountsApi.ReverseTransaction(ctx, transactionID, userID, opts)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("accounts: ReverseTransaction: transaction=%s: %v", transactionID, err)
	}
	return nil
}

// CreateAccountsClient returns an AccountsClient used to make HTTP calls over to a Account instance.
// By default moov's localhost bind address will be used or the Kubernetes DNS name
// when called from inside a Kubernetes cluster.
//
// endpoint is a DNS record responsible for routing us to an Account instance.
// Example: http://accounts.apps.svc.cluster.local:8080
func CreateAccountsClient(logger log.Logger, endpoint string, httpClient *http.Client) AccountsClient {
	conf := accounts.NewConfiguration()
	conf.HTTPClient = httpClient

	if endpoint != "" {
		conf.BasePath = endpoint
	} else {
		if k8s.Inside() {
			conf.BasePath = "http://accounts.apps.svc.cluster.local:8080"
		} else {
			conf.BasePath = "http://localhost" + bind.HTTP("accounts")
		}
	}
	logger.Log("accounts", fmt.Sprintf("using %s for Accounts address", conf.BasePath))

	return &moovAccountsClient{
		underlying: accounts.NewAPIClient(conf),
		logger:     logger,
	}
}

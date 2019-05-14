// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/base/docker"
	gl "github.com/moov-io/gl/client"

	"github.com/go-kit/kit/log"
	"github.com/ory/dockertest"
)

type testGLClient struct {
	accounts    []gl.Account
	transaction *gl.Transaction

	err error
}

func (c *testGLClient) Ping() error {
	return c.err
}

func (c *testGLClient) PostTransaction(_, _ string, lines []transactionLine) (*gl.Transaction, error) {
	if len(lines) == 0 {
		return nil, errors.New("no transactionLine's")
	}
	if c.err != nil {
		return nil, c.err
	}
	return c.transaction, nil
}

func (c *testGLClient) SearchAccounts(_, _ string, _ *Depository) (*gl.Account, error) {
	if c.err != nil {
		return nil, c.err
	}
	if len(c.accounts) > 0 {
		return &c.accounts[0], nil
	}
	return nil, nil
}

type glDeployment struct {
	res    *dockertest.Resource
	client GLClient
}

func (d *glDeployment) close(t *testing.T) {
	if err := d.res.Close(); err != nil {
		t.Error(err)
	}
}

func spawnGL(t *testing.T) *glDeployment {
	// no t.Helper() call so we know where it failed

	if testing.Short() {
		t.Skip("-short flag enabled")
	}
	if !docker.Enabled() {
		t.Skip("Docker not enabled")
	}

	// Spawn GL docker image
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "moov/gl",
		Tag:        "v0.2.3-dev",
		Cmd:        []string{"-http.addr=:8080"},
		Env: []string{
			"DEFAULT_ROUTING_NUMBER=121042882",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	client := createGLClient(log.NewNopLogger(), fmt.Sprintf("http://localhost:%s", resource.GetPort("8080/tcp")))
	err = pool.Retry(func() error {
		return client.Ping()
	})
	if err != nil {
		t.Fatal(err)
	}
	return &glDeployment{resource, client}
}

func TestGL__client(t *testing.T) {
	endpoint := ""
	if client := createGLClient(log.NewNopLogger(), endpoint); client == nil {
		t.Fatal("expected non-nil client")
	}

	// Spawn GL Docker image and ping against it
	deployment := spawnGL(t)
	if err := deployment.client.Ping(); err != nil {
		t.Fatal(err)
	}
	deployment.close(t) // close only if successful
}

func TestGL(t *testing.T) {
	deployment := spawnGL(t)
	client, ok := deployment.client.(*moovGLClient)
	if !ok {
		t.Fatalf("got %T", deployment.client)
	}

	userId := base.ID()

	// Create accounts behind the scenes
	fromAccount, err := createGLAccount(client, "from account", "Savings", userId)
	if err != nil {
		t.Fatal(err)
	}
	toAccount, err := createGLAccount(client, "to account", "Savings", userId)
	if err != nil {
		t.Fatal(err)
	}

	// Setup our Transaction
	lines := []transactionLine{
		{AccountId: toAccount.AccountId, Purpose: "achcredit", Amount: 10000},
		{AccountId: fromAccount.AccountId, Purpose: "achdebit", Amount: -10000},
	}
	tx, err := deployment.client.PostTransaction(base.ID(), userId, lines)
	if err != nil || tx == nil {
		t.Fatalf("transaction=%v error=%v", tx, err)
	}

	// Verify From Account
	account, err := deployment.client.SearchAccounts(base.ID(), userId, &Depository{
		ID:            DepositoryID(base.ID()),
		AccountNumber: fromAccount.AccountNumber,
		RoutingNumber: fromAccount.RoutingNumber,
		Type:          Savings,
	})
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 90000 { // $900
		t.Errorf("fromAccount balance: %d", account.Balance)
	}

	// Verify To Account
	account, err = deployment.client.SearchAccounts(base.ID(), userId, &Depository{
		ID:            DepositoryID(base.ID()),
		AccountNumber: toAccount.AccountNumber,
		RoutingNumber: toAccount.RoutingNumber,
		Type:          Savings,
	})
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 110000 { // $1100
		t.Errorf("fromAccount balance: %d", account.Balance)
	}

	deployment.close(t) // close only if successful
}

func createGLAccount(api *moovGLClient, name, tpe string, userId string) (*gl.Account, error) {
	ctx := context.TODO()
	req := gl.CreateAccount{Name: name, Type: tpe, Balance: 1000 * 100}

	account, resp, err := api.underlying.GLApi.CreateAccount(ctx, userId, userId, req, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("problem creating GL account %s: %v", name, err)
	}
	return &account, nil
}

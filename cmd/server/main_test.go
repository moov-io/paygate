// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"testing"

	"github.com/moov-io/base/admin"

	"github.com/go-kit/kit/log"
)

func TestMain__setupACHClient(t *testing.T) {
	logger := log.NewNopLogger()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	client := setupACHClient(logger, "", svc, httpClient)
	if client == nil {
		t.Error("expected ACH client")
	}
}

func TestMain__setupAccountsClient(t *testing.T) {
	logger := log.NewNopLogger()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	client := setupAccountsClient(logger, svc, httpClient, "", "yes")
	if client != nil {
		t.Errorf("expected disabled (nil) AccountsClient: %v", client)
	}
	client = setupAccountsClient(logger, svc, httpClient, "", "")
	if client == nil {
		t.Error("expected non-nil AccountsClient")
	}
}

func TestMain__setupCustomersClient(t *testing.T) {
	logger := log.NewNopLogger()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	client := setupCustomersClient(logger, svc, httpClient, "", "yes")
	if client != nil {
		t.Errorf("expected disabled (nil) customers.Client: %v", client)
	}

	client = setupCustomersClient(logger, svc, httpClient, "", "")
	if client == nil {
		t.Error("expected non-nil customers.Client")
	}
}

func TestMain__setupCustomerRefresher(t *testing.T) {
	logger := log.NewNopLogger()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	client := setupCustomersClient(logger, svc, httpClient, "", "")
	ref := setupCustomersRefresher(logger, client, nil)
	if ref == nil {
		t.Fatal("expected Customers refresher")
	}
	ref.Close()
}

func TestMain__setupFEDClient(t *testing.T) {
	logger := log.NewNopLogger()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	client := setupFEDClient(logger, "", svc, httpClient)
	if client == nil {
		t.Error("expected FED client")
	}
}

func TestMain__setupODFIAccount(t *testing.T) {
	logger := log.NewNopLogger()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	accountsClient := setupAccountsClient(logger, svc, httpClient, "", "")
	if accountsClient == nil {
		t.Fatal("expected an Accounts client")
	}

	acct := setupODFIAccount(accountsClient)
	if acct == nil {
		t.Error("expected ODFI account")
	}
}

func TestMain__setupACHStorageDir(t *testing.T) {
	if dir := setupACHStorageDir(log.NewNopLogger()); dir != "./storage/" {
		t.Errorf("unexpected ACH storage directory: %s", dir)
	}
}

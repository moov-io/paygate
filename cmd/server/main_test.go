// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/secrets"

	"github.com/go-kit/kit/log"
)

// TestMain starts paygate and verifies the system can come up without configuration.
//
// Also, we want to run Uber's goleak program to detect goroutine leaks within our application.
//
// TODO(adam): go.opencensus.io (transitive dependency) has a leak
// if upstream leak is fixed then we can enable this test.
// func TestMain(m *testing.M) {
// 	goleak.VerifyTestMain(m) // from go.uber.org/goleak
// }

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
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	cfg := config.Empty()
	cfg.Customers.Disabled = true

	client := setupCustomersClient(cfg, svc, httpClient)
	if client != nil {
		t.Errorf("expected disabled (nil) customers.Client: %v", client)
	}

	cfg.Customers.Disabled = false
	client = setupCustomersClient(cfg, svc, httpClient)
	if client == nil {
		t.Error("expected non-nil customers.Client")
	}
}

func TestMain__setupOFACRefresher(t *testing.T) {
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	cfg := config.Empty()
	cfg.Customers.OFACRefreshEvery = 1 * time.Minute

	client := setupCustomersClient(cfg, svc, httpClient)
	if client == nil {
		t.Error("expected non-nil customers Client")
	}
	ref := setupOFACRefresher(cfg, client, db.DB)
	if ref == nil {
		t.Fatal("expected Customers refresher")
	}
	ref.Close()
}

func TestMain__setupOFACRefresherNil(t *testing.T) {
	cfg := config.Empty()
	ref := setupOFACRefresher(cfg, nil, nil)
	if ref != nil {
		ref.Close()
		t.Errorf("expected nil Refresher: %T %#v", ref, ref)
	}
}

func TestMain__setupFEDClient(t *testing.T) {
	logger := log.NewNopLogger()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	client := setupFEDClient(logger, "", "", svc, httpClient)
	if client == nil {
		t.Error("expected FED client")
	}

	client = setupFEDClient(logger, "", "yes", svc, httpClient)
	if client != nil {
		t.Errorf("expected no FED client=%#v", client)
	}
}

func TestMain__setupODFIAccount(t *testing.T) {
	logger := log.NewNopLogger()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	keeper := secrets.TestStringKeeper(t)

	accountsClient := setupAccountsClient(logger, svc, httpClient, "", "")
	if accountsClient == nil {
		t.Fatal("expected an Accounts client")
	}

	acct := setupODFIAccount(accountsClient, keeper)
	if acct == nil {
		t.Error("expected ODFI account")
	}
}

func TestMain__setupACHStorageDir(t *testing.T) {
	dir := setupACHStorageDir(log.NewNopLogger())

	if dir != "./storage/" {
		t.Errorf("unexpected ACH storage directory: %s", dir)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
}

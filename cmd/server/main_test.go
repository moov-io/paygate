// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"os"
	"testing"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/internal/config"
)

func TestMain__setupACHClient(t *testing.T) {
	cfg := config.Empty()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	client := setupACHClient(cfg, svc, httpClient)
	if client == nil {
		t.Error("expected ACH client")
	}
}

func TestMain__setupAccountsClient(t *testing.T) {
	cfg := config.Empty()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	cfg.Accounts.Disabled = true
	client := setupAccountsClient(cfg, svc, httpClient)
	if client != nil {
		t.Errorf("expected disabled (nil) AccountsClient: %v", client)
	}

	cfg.Accounts.Disabled = false
	client = setupAccountsClient(cfg, svc, httpClient)
	if client == nil {
		t.Error("expected non-nil AccountsClient")
	}
}

func TestMain__setupFEDClient(t *testing.T) {
	cfg := config.Empty()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	client := setupFEDClient(cfg, svc, httpClient)
	if client == nil {
		t.Error("expected FED client")
	}
}

func TestMain__setupODFIAccount(t *testing.T) {
	cfg := config.Empty()
	svc := admin.NewServer(":0")
	httpClient := &http.Client{}

	accountsClient := setupAccountsClient(cfg, svc, httpClient)
	if accountsClient == nil {
		t.Fatal("expected an Accounts client")
	}

	cfg.ODFI = &config.ODFIConfig{
		AccountNumber:  "12345",
		AccountType:    "Checking",
		BankName:       "Moov Bank",
		Holder:         "Jane Smith",
		Identification: "21111111",
		RoutingNumber:  "987654320",
	}

	acct := setupODFIAccount(cfg, accountsClient)
	if acct == nil {
		t.Error("expected ODFI account")
	}
}

func TestMain__setupOFACClient(t *testing.T) {
	cfg := config.Empty()
	svc := admin.NewServer(":0")

	httpClient := &http.Client{}

	client := setupOFACClient(cfg, svc, httpClient)
	if client == nil {
		t.Error("expected OFAC client")
	}
}

func TestMain__setupACHStorageDir(t *testing.T) {
	defer os.RemoveAll("storage")

	cfg := config.Empty()
	setupACHStorageDir(cfg) // don't panic

	cfg.ACH = &config.ACHConfig{
		StorageDir: "./storage/",
	}
	setupACHStorageDir(cfg) // don't panic
}

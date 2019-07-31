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

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package features

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base/admin"
)

func TestRoutes(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	AddRoutes(log.NewNopLogger(), svc, true, false)

	resp, err := http.DefaultClient.Get("http://" + svc.BindAddr() + "/features")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var wrapper response
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		t.Error(err)
	}

	if !wrapper.AccountsCallsDisabled {
		t.Errorf("AccountsCallsDisabled=%v", wrapper.AccountsCallsDisabled)
	}
	if wrapper.CustomersCallsDisabled {
		t.Errorf("CustomersCallsDisabled=%v", wrapper.CustomersCallsDisabled)
	}

	// invalid HTTP method
	resp, err = http.DefaultClient.Post("http://"+svc.BindAddr()+"/features", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %s", resp.Status)
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"net/http"
	"testing"

	"github.com/moov-io/base/admin"

	"github.com/go-kit/kit/log"
)

func TestAdmin__mergeOutgoingFiles(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	flushOutgoing := make(FlushChan, 1)
	RegisterAdminRoutes(log.NewNopLogger(), svc, nil, flushOutgoing, getZeroFiles)

	req, err := http.NewRequest("POST", "http://"+svc.BindAddr()+"/files/merge", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}

	// wrong HTTP method
	req, err = http.NewRequest("GET", "http://"+svc.BindAddr()+"/files/merge", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}
}

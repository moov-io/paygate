// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package paygate

import (
	"net/http"
	"testing"

	"github.com/moov-io/base/admin"

	"github.com/go-kit/kit/log"
)

func TestForceFileUpload(t *testing.T) {
	svc := admin.NewServer(":0")
	go func(t *testing.T) {
		if err := svc.Listen(); err != nil && err != http.ErrServerClosed {
			t.Fatal(err)
		}
	}(t)
	defer svc.Shutdown()

	flushIncoming, flushOutgoing := make(chan struct{}, 1), make(chan struct{}, 1) // buffered channel
	AddFileTransferSyncRoute(log.NewNopLogger(), svc, flushIncoming, flushOutgoing)

	req, err := http.NewRequest("POST", "http://localhost"+svc.BindAddr()+"/files/flush", nil)
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

	// we need to read from this channel to ensure a message was sent
	// if there's no message the test will timeout
	<-flushIncoming
	<-flushOutgoing

	// use the wrong HTTP verb and get an error
	req, err = http.NewRequest("GET", "http://localhost"+svc.BindAddr()+"/files/flush", nil)
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

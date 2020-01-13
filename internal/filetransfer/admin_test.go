// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/moov-io/base/admin"

	"github.com/go-kit/kit/log"
)

func TestFlushIncomingFiles(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	flushIncoming := make(FlushChan, 1)
	AddFileTransferSyncRoute(log.NewNopLogger(), svc, flushIncoming, nil)

	// invalid request, wrong HTTP verb
	req, err := http.NewRequest("GET", "http://"+svc.BindAddr()+"/files/flush/incoming", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}

	// valid request
	req, err = http.NewRequest("POST", "http://"+svc.BindAddr()+"/files/flush/incoming", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}

	<-flushIncoming
}

func TestFlushOutgoingFiles(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	flushOutgoing := make(FlushChan, 1)
	AddFileTransferSyncRoute(log.NewNopLogger(), svc, nil, flushOutgoing)

	// invalid request, wrong HTTP verb
	req, err := http.NewRequest("GET", "http://"+svc.BindAddr()+"/files/flush/outgoing", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}

	// valid request
	req, err = http.NewRequest("POST", "http://"+svc.BindAddr()+"/files/flush/outgoing", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}

	<-flushOutgoing
}

func TestFlushFilesUpload(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	flushIncoming, flushOutgoing := make(FlushChan, 1), make(FlushChan, 1) // buffered channel
	AddFileTransferSyncRoute(log.NewNopLogger(), svc, flushIncoming, flushOutgoing)

	req, err := http.NewRequest("POST", "http://"+svc.BindAddr()+"/files/flush", nil)
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
	req, err = http.NewRequest("GET", "http://"+svc.BindAddr()+"/files/flush", nil)
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

func TestFlush__maybeWaiter(t *testing.T) {
	u, _ := url.Parse("http://localhost/files/flush?wait")

	req := maybeWaiter(&http.Request{URL: u})
	if req == nil {
		t.Fatal("nil periodicFileOperationsRequest")
	}
	if req.waiter == nil {
		t.Fatal("nil waiter")
	}

	// expect a nil waiter now
	u, _ = url.Parse("http://localhost/files/flush")
	req = maybeWaiter(&http.Request{URL: u})
	if req == nil {
		t.Fatal("nil periodicFileOperationsRequest")
	}
	if req.waiter != nil {
		t.Fatal("expected nil waiter")
	}
}

func TestFlush__maybeWait(t *testing.T) {
	req := &periodicFileOperationsRequest{
		waiter: make(chan struct{}, 1),
	}
	w := httptest.NewRecorder()
	go func() {
		req.waiter <- struct{}{} // signal completion
	}()
	if err := maybeWait(w, req); err != nil {
		t.Error(err)
	}
}

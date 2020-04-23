// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAdmin__maybeWaiter(t *testing.T) {
	u, _ := url.Parse("http://localhost/files/flush?wait")

	req := maybeWaiter(&http.Request{URL: u})
	if req == nil {
		t.Fatal("nil Request")
	}
	if req.Waiter == nil {
		t.Fatal("nil waiter")
	}

	// expect a nil waiter now
	u, _ = url.Parse("http://localhost/files/flush")
	req = maybeWaiter(&http.Request{URL: u})
	if req == nil {
		t.Fatal("nil Request")
	}
	if req.Waiter != nil {
		t.Fatal("expected nil waiter")
	}
}

func TestAdmin__maybeWait(t *testing.T) {
	req := &Request{
		Waiter: make(chan struct{}, 1),
	}
	w := httptest.NewRecorder()
	go func() {
		req.Waiter <- struct{}{} // signal completion
	}()
	if err := maybeWait(w, req); err != nil {
		t.Error(err)
	}
}

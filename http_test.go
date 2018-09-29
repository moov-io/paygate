// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"net/http/httptest"
	"testing"
)

func TestHttp__paygateResponseWriter(t *testing.T) {
	// missing x-user-id
	r := httptest.NewRequest("GET", "/testing", nil)
	r.Header.Set("x-user-id", "")

	w := httptest.NewRecorder()
	_, err := wrapResponseWriter(w, r, "testing")
	if err == nil {
		t.Error("expected error")
	}

	w.Flush()
	if w.Code != 403 {
		t.Errorf("got %d", w.Code)
	}

	// success with x-user-id
	r = httptest.NewRequest("GET", "/testing", nil)
	r.Header.Set("x-user-id", "my-user-id")

	w = httptest.NewRecorder()
	_, err = wrapResponseWriter(w, r, "testing")
	if err != nil {
		t.Error(err)
	}

	w.Flush()
	if w.Code != 200 {
		t.Errorf("got %d", w.Code)
	}
}

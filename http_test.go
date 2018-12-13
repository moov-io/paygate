// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func TestHttp__internalError(t *testing.T) {
	w := httptest.NewRecorder()
	internalError(w, errors.New("test"))
	w.Flush()

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d", w.Code)
	}
}

func TestHttp__addPingRoute(t *testing.T) {
	r := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	addPingRoute(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}
	if v := w.Body.String(); v != "PONG" {
		t.Errorf("got %q", v)
	}
}

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

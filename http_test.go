// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moov-io/base"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestHttp__internalError(t *testing.T) {
	w := httptest.NewRecorder()
	internalError(log.NewNopLogger(), w, errors.New("test"))
	w.Flush()

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d", w.Code)
	}
}

func TestHttp__addPingRoute(t *testing.T) {
	r := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	addPingRoute(log.NewNopLogger(), router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}
	if v := w.Body.String(); v != "PONG" {
		t.Errorf("got %q", v)
	}
}

func TestHTTP__idempotency(t *testing.T) {
	logger := log.NewNopLogger()

	router := mux.NewRouter()
	router.Methods("GET").Path("/test").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("PONG"))
	})

	key := base.ID()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("x-idempotency-key", key)
	req.Header.Set("x-user-id", base.ID())

	// mark the key as seen
	if seen := inmemIdempotentRecorder.SeenBefore(key); seen {
		t.Errorf("shouldn't have been seen before")
	}

	// make our request
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("got %d", w.Code)
	}

	// Key should be seen now
	if seen := inmemIdempotentRecorder.SeenBefore(key); !seen {
		t.Errorf("should have seen %q", key)
	}
}

func TestHTTP__cleanMetricsPath(t *testing.T) {
	if v := cleanMetricsPath("/v1/paygate/ping"); v != "v1-paygate-ping" {
		t.Errorf("got %q", v)
	}
	if v := cleanMetricsPath("/v1/paygate/customers/19636f90bc95779e2488b0f7a45c4b68958a2ddd"); v != "v1-paygate-customers" {
		t.Errorf("got %q", v)
	}
	// A value which looks like moov/base.ID, but is off by one character (last letter)
	if v := cleanMetricsPath("/v1/paygate/customers/19636f90bc95779e2488b0f7a45c4b68958a2ddz"); v != "v1-paygate-customers-19636f90bc95779e2488b0f7a45c4b68958a2ddz" {
		t.Errorf("got %q", v)
	}
}

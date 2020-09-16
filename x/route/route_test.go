// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package route

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/config"

	"github.com/gorilla/mux"
)

func TestNamespace(t *testing.T) {
	req, err := http.NewRequest("GET", "http://moov.io/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Namespace", "foo")

	cfg := config.Empty()
	namespace := findNamespace(cfg.Namespace, req)
	if namespace != "foo" {
		t.Errorf("got %q", namespace)
	}

	// blank out
	cfg.Namespace.Default = "bar"
	req.Header.Set("X-Namespace", "")
	namespace = findNamespace(cfg.Namespace, req)
	if namespace != "bar" {
		t.Errorf("got %q", namespace)
	}
}

func TestRoute(t *testing.T) {
	cfg := config.Empty()

	router := mux.NewRouter()
	router.Methods("GET").Path("/test").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder := NewResponder(cfg, w, r)
		responder.Log("test", "response")
		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"error": null}`))
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Namespace", base.ID())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}
}

func TestRoute__problem(t *testing.T) {
	cfg := config.Empty()

	router := mux.NewRouter()
	router.Methods("GET").Path("/bad").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder := NewResponder(cfg, w, r)
		responder.Problem(errors.New("bad error"))
	})

	req := httptest.NewRequest("GET", "/bad", nil)
	req.Header.Set("X-Namespace", base.ID())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestRoute__Idempotency(t *testing.T) {
	cfg := config.Empty()

	router := mux.NewRouter()
	router.Methods("GET").Path("/test").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder := NewResponder(cfg, w, r)
		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("PONG"))
		})
	})

	key := base.ID()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("x-idempotency-key", key)
	req.Header.Set("X-Namespace", base.ID())

	// mark the key as seen
	if seen := IdempotentRecorder.SeenBefore(key); seen {
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
	if seen := IdempotentRecorder.SeenBefore(key); !seen {
		t.Errorf("should have seen %q", key)
	}
}

func TestRoute__CleanPath(t *testing.T) {
	if v := CleanPath("/v1/paygate/ping"); v != "v1-paygate-ping" {
		t.Errorf("got %q", v)
	}
	if v := CleanPath("/v1/paygate/customers/19636f90bc95779e2488b0f7a45c4b68958a2ddd"); v != "v1-paygate-customers" {
		t.Errorf("got %q", v)
	}
	// A value which looks like moov/base.ID, but is off by one character (last letter)
	if v := CleanPath("/v1/paygate/customers/19636f90bc95779e2488b0f7a45c4b68958a2ddz"); v != "v1-paygate-customers-19636f90bc95779e2488b0f7a45c4b68958a2ddz" {
		t.Errorf("got %q", v)
	}
}

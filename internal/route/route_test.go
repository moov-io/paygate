// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package route

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestResponderNoUserID(t *testing.T) {
	logger := log.NewNopLogger()

	router := mux.NewRouter()
	router.Methods("GET").Path("/bad").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder := NewResponder(logger, w, r)
		responder.Problem(errors.New("bad error"))
	})

	req := httptest.NewRequest("GET", "/bad", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no X-User-Id header provided") {
		t.Errorf("body: %s", w.Body.String())
	}
}

func TestHeaderUserID(t *testing.T) {
	req, err := http.NewRequest("GET", "http://moov.io/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-user-id", "foo")

	userID := HeaderUserID(req).String()
	if userID != "foo" {
		t.Errorf("got %q", userID)
	}
}

func TestPathUserID(t *testing.T) {
	var userID id.User

	r := mux.NewRouter()
	r.Methods("GET").Path("/users/{userId}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID = PathUserID(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/users/foo", nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if u := userID.String(); u != "foo" {
		t.Errorf("got %q", u)
	}
}

func TestRoute(t *testing.T) {
	logger := log.NewNopLogger()

	router := mux.NewRouter()
	router.Methods("GET").Path("/test").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder := NewResponder(logger, w, r)
		responder.Log("test", "response")
		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"error": null}`))
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("x-user-id", base.ID())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}
}

func TestRoute__problem(t *testing.T) {
	logger := log.NewNopLogger()

	router := mux.NewRouter()
	router.Methods("GET").Path("/bad").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder := NewResponder(logger, w, r)
		responder.Problem(errors.New("bad error"))
	})

	req := httptest.NewRequest("GET", "/bad", nil)
	req.Header.Set("x-user-id", base.ID())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestRoute__Idempotency(t *testing.T) {
	logger := log.NewNopLogger()

	router := mux.NewRouter()
	router.Methods("GET").Path("/test").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder := NewResponder(logger, w, r)
		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("PONG"))
		})
	})

	key := base.ID()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("x-idempotency-key", key)
	req.Header.Set("x-user-id", base.ID())

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

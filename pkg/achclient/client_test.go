// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

var (
	addPingRoute = func(r *mux.Router) {
		r.Methods("GET").Path("/ping").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("PONG"))
			w.WriteHeader(http.StatusOK)
		})
	}
	addCreateRoute = func(r *mux.Router) {
		r.Methods("POST").Path("/create").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v := r.Header.Get("X-Idempotency-Key"); v != "" {
				// copy header to response (for tests)
				w.Header().Set("X-Idempotency-Key", v)
			}
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("create"))
			w.WriteHeader(http.StatusOK)
		})
	}
)

func newACHWithClientServer(name string, routes ...func(*mux.Router)) (*ACH, *http.Client, *httptest.Server) {
	r := mux.NewRouter()
	for i := range routes {
		routes[i](r) // Add each route
	}
	server := httptest.NewServer(r)
	client := server.Client()

	achClient := New(name, log.NewNopLogger())
	achClient.client = client
	achClient.endpoint = server.URL

	return achClient, client, server
}

func TestACH__pingRoute(t *testing.T) {
	achClient, _, server := newACHWithClientServer("pingRoute", addPingRoute)
	defer server.Close()

	// Make our ping request
	if err := achClient.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestACH__post(t *testing.T) {
	achClient, _, server := newACHWithClientServer("post", addCreateRoute)
	defer server.Close()

	resp, err := achClient.POST("/create", "unique", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if v := resp.Header.Get("X-Idempotency-Key"); v != "unique" {
		t.Error(v)
	}
	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	if v := string(bs); v != "create" {
		t.Error(v)
	}
}

func TestACH__buildAddress(t *testing.T) {
	achClient := &ACH{
		endpoint: "http://localhost:8080",
	}
	if v := achClient.buildAddress("/ping"); v != "http://localhost:8080/ping" {
		t.Errorf("got %q", v)
	}

	achClient.endpoint = "http://localhost:8080/"
	if v := achClient.buildAddress("/ping"); v != "http://localhost:8080/ping" {
		t.Errorf("got %q", v)
	}

	achClient.endpoint = "https://api.moov.io/v1/ach"
	if v := achClient.buildAddress("/ping"); v != "https://api.moov.io/v1/ach/ping" {
		t.Errorf("got %q", v)
	}
}

func TestACH__addRequestHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/ping", nil)
	api := New("addRequestHeaders", log.NewNopLogger())
	api.addRequestHeaders("idempotencyKey", "requestId", req)

	if v := req.Header.Get("User-Agent"); !strings.HasPrefix(v, "ach/") {
		t.Errorf("got %q", v)
	}
	if v := req.Header.Get("X-Request-Id"); v == "" {
		t.Error("empty header value")
	}
}

func TestACH__retryWait(t *testing.T) {
	neg := -1 * time.Millisecond
	var cases = map[int]time.Duration{
		-99: neg,
		-1:  neg,
		0:   10 * time.Millisecond,
		1:   15 * time.Millisecond,
		2:   25 * time.Millisecond,
		3:   45 * time.Millisecond,
		4:   85 * time.Millisecond,
		5:   neg,
		6:   neg,
		100: neg,
	}
	ach := New("retryWait", log.NewNopLogger())
	for n, expected := range cases {
		ans := ach.retryWait(n)
		if expected != ans {
			t.Errorf("n=%d, got %s, but expected %s", n, ans, expected)
		}
	}
}

func TestACH__retry(t *testing.T) {
	fails := 0
	handler := func(r *mux.Router) {
		r.Methods("GET").Path("/test").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if fails > 1 {
				w.WriteHeader(http.StatusOK)
				return
			}
			fails += 1
			w.WriteHeader(http.StatusInternalServerError)
		})
	}
	achClient, _, server := newACHWithClientServer("retry", handler)
	defer server.Close()

	// make our request
	resp, err := achClient.GET("/test")
	if err != nil {
		t.Error(err)
	}
	resp.Body.Close()

	// verify attempts
	if fails != 2 {
		t.Errorf("fails=%d", fails)
	}
}

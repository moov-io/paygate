// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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

// TestACH__getACHAddress will fail if ever ran inside a Kubernetes cluster.
func TestACH__getACHAddress(t *testing.T) {
	// Local development
	if addr := getACHAddress(); addr != "http://localhost:8080" {
		t.Error(addr)
	}

	// ACH_ENDPOINT environment variable
	os.Setenv("ACH_ENDPOINT", "https://api.moov.io/v1/ach")
	if addr := getACHAddress(); addr != "https://api.moov.io/v1/ach" {
		t.Error(addr)
	}
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
	req, err := http.NewRequest("GET", "/ping", nil)
	if err != nil {
		t.Fatal(err)
	}

	api := New("addRequestHeaders", log.NewNopLogger())
	api.addRequestHeaders("idempotencyKey", "requestId", req)

	if v := req.Header.Get("User-Agent"); !strings.HasPrefix(v, "ach/") {
		t.Errorf("got %q", v)
	}
	if v := req.Header.Get("X-Request-Id"); v == "" {
		t.Error("empty header value")
	}
}

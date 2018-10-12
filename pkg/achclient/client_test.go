// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"strings"
)

var (
	addPingRoute = func(r *mux.Router) {
		r.Methods("GET").Path("/ping").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("PONG"))
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
	api.addRequestHeaders(req)

	if v := req.Header.Get("User-Agent"); !strings.HasPrefix(v, "ach/") {
		t.Errorf("got %q", v)
	}
	if v := req.Header.Get("X-Request-Id"); v == "" {
		t.Error("empty header value")
	}
}

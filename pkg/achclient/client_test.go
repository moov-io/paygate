// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestACH__pingRoute(t *testing.T) {
	achClient, _, server := MockClientServer("pingRoute", AddPingRoute)
	defer server.Close()

	// Make our ping request
	if err := achClient.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestACH__delete(t *testing.T) {
	achClient, _, server := MockClientServer("delete", AddDeleteRoute)
	defer server.Close()

	resp, err := achClient.DELETE("/files/delete")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// verify we hit the 'DELETE /test' route
	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	if v := string(bs); v != "{}" {
		t.Error(v)
	}
}

func TestACH__post(t *testing.T) {
	achClient, _, server := MockClientServer("post", func(r *mux.Router) { AddCreateRoute(nil, r) })
	defer server.Close()

	body := strings.NewReader(`{"id": "foo"}`) // partial ach.File JSON

	resp, err := achClient.POST("/files/create", "unique", ioutil.NopCloser(body))
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
	if v := string(bs); !strings.HasPrefix(v, `{"id":`) {
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

	api := New(log.NewNopLogger(), "", "addRequestHeaders", nil)
	api.addRequestHeaders("idempotencyKey", "requestID", req)

	if v := req.Header.Get("User-Agent"); !strings.HasPrefix(v, "ach/") {
		t.Errorf("got %q", v)
	}
	if v := req.Header.Get("X-Request-Id"); v == "" {
		t.Error("empty header value")
	}
}

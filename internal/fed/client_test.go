// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fed

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/log"
)

func TestFED(t *testing.T) {
	svc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`PONG`))
	}))

	client := NewClient(log.NewNopLogger(), svc.URL, nil)
	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}
	svc.Close()

	// test LookupRoutingNumber
	svc = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fed/ach/search" {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"achParticipants": [{"routingNumber": "121042882"}]}`)) // partial fed.AchDictionary response
	}))

	client = NewClient(log.NewNopLogger(), svc.URL, nil)
	if err := client.LookupRoutingNumber("121042882"); err != nil {
		t.Fatal(err)
	}
	svc.Close()
}

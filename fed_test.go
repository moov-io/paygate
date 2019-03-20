// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
)

func TestFED__Ping(t *testing.T) {
	svc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`PONG`))
	}))
	os.Setenv("FED_ENDPOINT", svc.URL)
	defer svc.Close()

	client := createFEDClient(log.NewNopLogger())
	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}
}

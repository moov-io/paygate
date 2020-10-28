// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package route

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/moov-io/base/log"
)

func TestPingRoute(t *testing.T) {
	router := mux.NewRouter()
	PingRoute(log.NewNopLogger(), router)

	req := httptest.NewRequest("GET", "/ping", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d", w.Code)
	}
	if v := w.Body.String(); v != "PONG" {
		t.Errorf("unexpected response: %s", v)
	}
}

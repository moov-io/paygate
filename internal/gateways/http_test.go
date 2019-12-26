// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gateways

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
)

func TestGateways__gatewayRequest(t *testing.T) {
	req := gatewayRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestGateways__HTTPGetNoUserID(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLGatewayRepo{db.DB, log.NewNopLogger()}

	router := mux.NewRouter()
	AddRoutes(log.NewNopLogger(), router, repo)

	body := strings.NewReader(`{"key": "value"}`)
	req := httptest.NewRequest("GET", "/gateways", body)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status=%d: %v", w.Code, w.Body.String())
	}
}

func TestGateways__HTTPCreate(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLGatewayRepo{db.DB, log.NewNopLogger()}

	router := mux.NewRouter()
	AddRoutes(log.NewNopLogger(), router, repo)

	body := strings.NewReader(`{"origin": "987654320", "originName": "bank", "destination": "123456780", "destinationName": "other bank"}`)
	req := httptest.NewRequest("POST", "/gateways", body)
	req.Header.Set("x-user-id", base.ID())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status=%d: %v", w.Code, w.Body.String())
	}

	var wrapper struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
		t.Fatal(err)
	}
	if wrapper.ID == "" {
		t.Errorf("missing ID: %v", w.Body.String())
	}
}

func TestGateways__HTTPCreateNoUserID(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLGatewayRepo{db.DB, log.NewNopLogger()}

	router := mux.NewRouter()
	AddRoutes(log.NewNopLogger(), router, repo)

	body := strings.NewReader(`{"key": "value"}`)
	req := httptest.NewRequest("POST", "/gateways", body)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status=%d: %v", w.Code, w.Body.String())
	}
}

func TestGateways__HTTPCreateErr(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLGatewayRepo{db.DB, log.NewNopLogger()}

	router := mux.NewRouter()
	AddRoutes(log.NewNopLogger(), router, repo)

	// invalid JSON
	body := strings.NewReader(`{...}`)
	req := httptest.NewRequest("POST", "/gateways", body)
	req.Header.Set("x-user-id", base.ID())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status=%d: %v", w.Code, w.Body.String())
	}
}

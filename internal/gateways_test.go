// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

func TestGateways__gatewayRequest(t *testing.T) {
	req := gatewayRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestGateways_getUserGateways(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo gatewayRepository) {
		userID := id.User(base.ID())
		req := gatewayRequest{
			Origin:          "231380104",
			OriginName:      "my bank",
			Destination:     "031300012",
			DestinationName: "my other bank",
		}
		gateway, err := repo.createUserGateway(userID, req)
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/gateways", nil)
		r.Header.Set("x-user-id", userID.String())

		getUserGateway(log.NewNopLogger(), repo)(w, r)
		w.Flush()

		if w.Code != 200 {
			t.Errorf("got %d", w.Code)
		}

		var gw *Gateway
		if err := json.Unmarshal(w.Body.Bytes(), &gw); err != nil {
			t.Error(err)
		}
		if gw.ID != gateway.ID {
			t.Errorf("gw.ID=%v, gateway.ID=%v", gw.ID, gateway.ID)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLGatewayRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLGatewayRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestGateways_update(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo gatewayRepository) {
		userID := id.User(base.ID())
		req := gatewayRequest{
			Origin:          "231380104",
			OriginName:      "my bank",
			Destination:     "031300012",
			DestinationName: "my other bank",
		}
		gateway, err := repo.createUserGateway(userID, req)
		if err != nil {
			t.Fatal(err)
		}

		// read gateway
		gw, err := repo.getUserGateway(userID)
		if err != nil {
			t.Fatal(err)
		}
		if gw.ID != gateway.ID {
			t.Errorf("gw.ID=%v gateway.ID=%v", gw.ID, gateway.ID)
		}

		// Update Origin
		req.Origin = "031300012"
		_, err = repo.createUserGateway(userID, req)
		if err != nil {
			t.Fatal(err)
		}
		gw, err = repo.getUserGateway(userID)
		if err != nil {
			t.Fatal(err)
		}
		if gw.ID != gateway.ID {
			t.Errorf("gw.ID=%v gateway.ID=%v", gw.ID, gateway.ID)
		}
		if gw.Origin != req.Origin {
			t.Errorf("gw.Origin=%v expected %v", gw.Origin, req.Origin)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLGatewayRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLGatewayRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestGateways__HTTPCreate(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLGatewayRepo{db.DB, log.NewNopLogger()}

	router := mux.NewRouter()
	AddGatewayRoutes(log.NewNopLogger(), router, repo)

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

func TestGateways__HTTPCreateErr(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLGatewayRepo{db.DB, log.NewNopLogger()}

	router := mux.NewRouter()
	AddGatewayRoutes(log.NewNopLogger(), router, repo)

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

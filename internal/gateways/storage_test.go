// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gateways

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

func TestGateways_getUserGateways(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *SQLGatewayRepo) {
		defer repo.Close()

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

		if w.Code != http.StatusOK {
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
	check(t, NewRepo(log.NewNopLogger(), sqliteDB.DB))

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, NewRepo(log.NewNopLogger(), mysqlDB.DB))
}

func TestGateways_update(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
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

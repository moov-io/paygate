// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package originators

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

func TestOriginators__originatorRequest(t *testing.T) {
	req := originatorRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestOriginators_getUserOriginators(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
		userID := id.User(base.ID())
		req := originatorRequest{
			DefaultDepository: "depository",
			Identification:    "secret value",
			Metadata:          "extra data",
			customerID:        "custID",
		}
		orig, err := repo.createUserOriginator(userID, req)
		if err != nil {
			t.Fatal(err)
		}
		if orig.CustomerID != "custID" {
			t.Errorf("orig.CustomerID=%s", orig.CustomerID)
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/originators", nil)
		r.Header.Set("x-user-id", userID.String())

		getUserOriginators(log.NewNopLogger(), repo)(w, r)
		w.Flush()

		if w.Code != http.StatusOK {
			t.Errorf("got %d", w.Code)
		}

		var originators []*model.Originator
		if err := json.Unmarshal(w.Body.Bytes(), &originators); err != nil {
			t.Error(err)
		}
		if len(originators) != 1 {
			t.Errorf("got %d originators=%v", len(originators), originators)
		}
		if originators[0].ID == "" {
			t.Errorf("originators[0]=%v", originators[0])
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLOriginatorRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLOriginatorRepo{mysqlDB.DB, log.NewNopLogger()})
}

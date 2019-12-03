// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accounts "github.com/moov-io/accounts/client"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type mockOriginatorRepository struct {
	originators []*Originator
	err         error
}

func (r *mockOriginatorRepository) getUserOriginators(userID id.User) ([]*Originator, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.originators, nil
}

func (r *mockOriginatorRepository) getUserOriginator(id OriginatorID, userID id.User) (*Originator, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(r.originators) > 0 {
		return r.originators[0], nil
	}
	return nil, nil
}

func (r *mockOriginatorRepository) createUserOriginator(userID id.User, req originatorRequest) (*Originator, error) {
	if len(r.originators) > 0 {
		return r.originators[0], nil
	}
	return nil, nil
}

func (r *mockOriginatorRepository) deleteUserOriginator(id OriginatorID, userID id.User) error {
	return r.err
}

func TestOriginators__read(t *testing.T) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(originatorRequest{
		DefaultDepository: id.Depository("test"),
		Identification:    "secret",
		Metadata:          "extra",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := readOriginatorRequest(&http.Request{
		Body: ioutil.NopCloser(&buf),
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.DefaultDepository != "test" {
		t.Error(req.DefaultDepository)
	}
	if req.Identification != "secret" {
		t.Error(req.Identification)
	}
	if req.Metadata != "extra" {
		t.Error(req.Metadata)
	}
}

func TestOriginators__originatorRequest(t *testing.T) {
	req := originatorRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestOriginators_getUserOriginators(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo originatorRepository) {
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

		if w.Code != 200 {
			t.Errorf("got %d", w.Code)
		}

		var originators []*Originator
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

func TestOriginators_CustomersError(t *testing.T) {
	logger := log.NewNopLogger()

	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	keeper := secrets.TestStringKeeper(t)
	depRepo := NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)
	origRepo := &SQLOriginatorRepo{db.DB, log.NewNopLogger()}

	// Write Depository to repo
	userID := id.User(base.ID())
	dep := &Depository{
		ID:                     id.Depository(base.ID()),
		BankName:               "bank name",
		Holder:                 "holder",
		HolderType:             Individual,
		Type:                   Checking,
		RoutingNumber:          "123",
		EncryptedAccountNumber: "151",
		Status:                 DepositoryUnverified,
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	rawBody := fmt.Sprintf(`{"defaultDepository": "%s", "identification": "test@example.com", "metadata": "Jane Doe"}`, dep.ID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/originators", strings.NewReader(rawBody))
	req.Header.Set("x-user-id", userID.String())

	// happy path
	accountsClient := &testAccountsClient{
		accounts: []accounts.Account{
			{
				ID:            base.ID(),
				AccountNumber: dep.EncryptedAccountNumber,
				RoutingNumber: dep.RoutingNumber,
				Type:          "Checking",
			},
		},
	}

	customersClient := &customers.TestClient{}
	createUserOriginator(logger, accountsClient, customersClient, depRepo, origRepo)(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
	}

	// reset and block
	w = httptest.NewRecorder()
	customersClient = &customers.TestClient{
		Err: errors.New("bad error"),
	}
	req.Body = ioutil.NopCloser(strings.NewReader(rawBody))

	createUserOriginator(logger, accountsClient, customersClient, depRepo, origRepo)(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
	}
}

func TestOriginators_HTTPGet(t *testing.T) {
	userID, now := base.ID(), time.Now()
	orig := &Originator{
		ID:                OriginatorID(base.ID()),
		DefaultDepository: id.Depository(base.ID()),
		Identification:    "id",
		Metadata:          "other",
		Created:           base.NewTime(now),
		Updated:           base.NewTime(now),
	}
	repo := &mockOriginatorRepository{
		originators: []*Originator{orig},
	}

	router := mux.NewRouter()
	AddOriginatorRoutes(log.NewNopLogger(), router, nil, nil, nil, repo)

	req := httptest.NewRequest("GET", fmt.Sprintf("/originators/%s", orig.ID), nil)
	req.Header.Set("x-user-id", userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	var originator Originator
	if err := json.NewDecoder(w.Body).Decode(&originator); err != nil {
		t.Error(err)
	}
	if originator.ID != orig.ID {
		t.Errorf("unexpected originator: %s", originator.ID)
	}
}

func TestOriginators__HTTPGetAllNoUserID(t *testing.T) {
	repo := &mockOriginatorRepository{}

	router := mux.NewRouter()
	AddOriginatorRoutes(log.NewNopLogger(), router, nil, nil, nil, repo)

	req := httptest.NewRequest("GET", "/originators", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestOriginators__HTTPGetNoUserID(t *testing.T) {
	repo := &mockOriginatorRepository{}

	router := mux.NewRouter()
	AddOriginatorRoutes(log.NewNopLogger(), router, nil, nil, nil, repo)

	req := httptest.NewRequest("GET", "/originators/foo", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestOriginators__HTTPPost(t *testing.T) {
	userID := id.User(base.ID())

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	keeper := secrets.TestStringKeeper(t)
	depRepo := NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper)
	origRepo := &mockOriginatorRepository{}

	if err := depRepo.UpsertUserDepository(userID, &Depository{
		ID:            id.Depository("foo"),
		RoutingNumber: "987654320",
		Type:          Checking,
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Status:        DepositoryUnverified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
		keeper:        keeper,
	}); err != nil {
		t.Fatal(err)
	}

	router := mux.NewRouter()
	AddOriginatorRoutes(log.NewNopLogger(), router, nil, nil, depRepo, origRepo)

	body := strings.NewReader(`{"defaultDepository": "foo", "identification": "baz", "metadata": "other"}`)
	req := httptest.NewRequest("POST", "/originators", body)
	req.Header.Set("x-user-id", userID.String())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	var wrapper Originator
	if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
		t.Fatal(err)
	}
	if wrapper.ID != "" {
		t.Errorf("wrapper.ID=%s", wrapper.ID)
	}
}

func TestOriginators__HTTPPostNoUserID(t *testing.T) {
	repo := &mockOriginatorRepository{}

	router := mux.NewRouter()
	AddOriginatorRoutes(log.NewNopLogger(), router, nil, nil, nil, repo)

	req := httptest.NewRequest("POST", "/originators", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestOriginators__HTTPDelete(t *testing.T) {
	userID, now := base.ID(), time.Now()
	orig := &Originator{
		ID:                OriginatorID(base.ID()),
		DefaultDepository: id.Depository(base.ID()),
		Identification:    "id",
		Metadata:          "other",
		Created:           base.NewTime(now),
		Updated:           base.NewTime(now),
	}
	repo := &mockOriginatorRepository{
		originators: []*Originator{orig},
	}

	router := mux.NewRouter()
	AddOriginatorRoutes(log.NewNopLogger(), router, nil, nil, nil, repo)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/originators/%s", orig.ID), nil)
	req.Header.Set("x-user-id", userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestOriginators__HTTPDeleteNoUserID(t *testing.T) {
	repo := &mockOriginatorRepository{}

	router := mux.NewRouter()
	AddOriginatorRoutes(log.NewNopLogger(), router, nil, nil, nil, repo)

	req := httptest.NewRequest("DELETE", "/originators/foo", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
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
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type mockOriginatorRepository struct {
	originators []*Originator
	err         error
}

func (r *mockOriginatorRepository) getUserOriginators(userID string) ([]*Originator, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.originators, nil
}

func (r *mockOriginatorRepository) getUserOriginator(id OriginatorID, userID string) (*Originator, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(r.originators) > 0 {
		return r.originators[0], nil
	}
	return nil, nil
}

func (r *mockOriginatorRepository) createUserOriginator(userID string, req originatorRequest) (*Originator, error) {
	if len(r.originators) > 0 {
		return r.originators[0], nil
	}
	return nil, nil
}

func (r *mockOriginatorRepository) deleteUserOriginator(id OriginatorID, userID string) error {
	return r.err
}

func TestOriginators__read(t *testing.T) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(originatorRequest{
		DefaultDepository: DepositoryID("test"),
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
		userID := base.ID()
		req := originatorRequest{
			DefaultDepository: "depository",
			Identification:    "secret value",
			Metadata:          "extra data",
		}
		_, err := repo.createUserOriginator(userID, req)
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/originators", nil)
		r.Header.Set("x-user-id", userID)

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
	check(t, &sqliteOriginatorRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &sqliteOriginatorRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestOriginators_OFACMatch(t *testing.T) {
	logger := log.NewNopLogger()

	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	depRepo := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}
	origRepo := &sqliteOriginatorRepo{db.DB, log.NewNopLogger()}

	keeper := testSecretKeeper(testSecretKey)

	depID, userID := base.ID(), base.ID()
	// Write Depository to repo
	dep := &Depository{
		ID:                  DepositoryID(depID),
		BankName:            "bank name",
		Holder:              "holder",
		HolderType:          Individual,
		Type:                Checking,
		RoutingNumber:       "123",
		Status:              DepositoryUnverified,
		maskedAccountNumber: base64.StdEncoding.EncodeToString([]byte("#2151")),
	}
	if enc, err := encryptAccountNumber(keeper, dep, "151"); err != nil {
		t.Fatal(err)
	} else {
		dep.encryptedAccountNumber = enc
	}
	if err := depRepo.upsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	// Encrypt account number
	if k, err := keeper(fmt.Sprintf("depository-%s-account-number", depID)); err != nil {
		t.Fatal(err)
	} else {
		enc, err := k.Encrypt(context.Background(), []byte("32151"))
		if err != nil {
			t.Fatal(err)
		}
		dep.encryptedAccountNumber = base64.StdEncoding.EncodeToString(enc)
	}

	rawBody := fmt.Sprintf(`{"defaultDepository": "%s", "identification": "test@example.com", "metadata": "Jane Doe"}`, dep.ID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/originators", strings.NewReader(rawBody))
	req.Header.Set("x-user-id", userID)

	// happy path, no OFAC match
	accountsClient := &testAccountsClient{
		accounts: []accounts.Account{
			{
				ID:            base.ID(),
				AccountNumber: "1321",
				RoutingNumber: dep.RoutingNumber,
				Type:          "Checking",
			},
		},
	}
	ofacClient := &testOFACClient{}
	createUserOriginator(logger, keeper, false, accountsClient, ofacClient, origRepo, depRepo)(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
	}

	// reset and block via OFAC
	w = httptest.NewRecorder()
	ofacClient = &testOFACClient{
		err: errors.New("blocking"),
	}
	req.Body = ioutil.NopCloser(strings.NewReader(rawBody))
	createUserOriginator(logger, testSecretKeeper(testSecretKey), false, accountsClient, ofacClient, origRepo, depRepo)(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
	} else {
		if !strings.Contains(w.Body.String(), `ofac: blocking \"Jane Doe\"`) {
			t.Errorf("unknown error: %v", w.Body.String())
		}
	}
}

func TestOriginators_HTTPGet(t *testing.T) {
	userID, now := base.ID(), time.Now()
	orig := &Originator{
		ID:                OriginatorID(base.ID()),
		DefaultDepository: DepositoryID(base.ID()),
		Identification:    "id",
		Metadata:          "other",
		Created:           base.NewTime(now),
		Updated:           base.NewTime(now),
	}
	repo := &mockOriginatorRepository{
		originators: []*Originator{orig},
	}

	router := mux.NewRouter()
	addOriginatorRoutes(log.NewNopLogger(), router, testSecretKeeper(testSecretKey), true, nil, nil, nil, repo)

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

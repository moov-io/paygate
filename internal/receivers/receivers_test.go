// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package receivers

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

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestReceivers__read(t *testing.T) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(receiverRequest{
		Email:             "test@moov.io",
		DefaultDepository: id.Depository("test"),
		Metadata:          "extra",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := readReceiverRequest(&http.Request{
		Body: ioutil.NopCloser(&buf),
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Email != "test@moov.io" {
		t.Errorf("got %s", req.Email)
	}
	if req.DefaultDepository != "test" {
		t.Errorf("got %s", req.DefaultDepository)
	}
	if req.Metadata != "extra" {
		t.Errorf("got %s", req.Metadata)
	}
}
func TestReceivers__receiverRequest(t *testing.T) {
	req := receiverRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestReceivers__parseAndValidateEmail(t *testing.T) {
	if addr, err := parseAndValidateEmail("a@foo.com"); addr != "a@foo.com" || err != nil {
		t.Errorf("addr=%s error=%v", addr, err)
	}
	if addr, err := parseAndValidateEmail("a+bar@foo.com"); addr != "a+bar@foo.com" || err != nil {
		t.Errorf("addr=%s error=%v", addr, err)
	}
	if addr, err := parseAndValidateEmail(`"a b"@foo.com`); addr != `a b@foo.com` || err != nil {
		t.Errorf("addr=%s error=%v", addr, err)
	}
	if addr, err := parseAndValidateEmail("Barry Gibbs <bg@example.com>"); addr != "bg@example.com" || err != nil {
		t.Errorf("addr=%s error=%v", addr, err)
	}

	// sad path
	if addr, err := parseAndValidateEmail(""); addr != "" || err == nil {
		t.Errorf("addr=%s error=%v", addr, err)
	}
	if addr, err := parseAndValidateEmail("@"); addr != "" || err == nil {
		t.Errorf("addr=%s error=%v", addr, err)
	}
	if addr, err := parseAndValidateEmail("example.com"); addr != "" || err == nil {
		t.Errorf("addr=%s error=%v", addr, err)
	}
}

func TestReceivers__HTTPGet(t *testing.T) {
	userID, now := base.ID(), time.Now()
	rec := &model.Receiver{
		ID:                model.ReceiverID(base.ID()),
		Email:             "foo@moov.io",
		DefaultDepository: id.Depository(base.ID()),
		Status:            model.ReceiverVerified,
		Metadata:          "other",
		Created:           base.NewTime(now),
		Updated:           base.NewTime(now),
	}
	repo := &MockRepository{
		Receivers: []*model.Receiver{rec},
	}

	router := mux.NewRouter()
	AddReceiverRoutes(log.NewNopLogger(), router, nil, nil, repo)

	req := httptest.NewRequest("GET", fmt.Sprintf("/receivers/%s", rec.ID), nil)
	req.Header.Set("x-user-id", userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	var receiver model.Receiver
	if err := json.NewDecoder(w.Body).Decode(&receiver); err != nil {
		t.Error(err)
	}
	if receiver.ID != rec.ID {
		t.Errorf("unexpected receiver: %s", receiver.ID)
	}
}

func TestReceivers__HTTPGetNoUserID(t *testing.T) {
	repo := &MockRepository{}

	router := mux.NewRouter()
	AddReceiverRoutes(log.NewNopLogger(), router, nil, nil, repo)

	req := httptest.NewRequest("GET", "/receivers/foo", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestReceivers__HTTPUpdate(t *testing.T) {
	now := time.Now()
	receiverID, userID := model.ReceiverID(base.ID()), id.User(base.ID())

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	keeper := secrets.TestStringKeeper(t)
	depRepo := depository.NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper)
	dep := &model.Depository{
		ID:                     id.Depository(base.ID()),
		BankName:               "bank name",
		Holder:                 "holder",
		HolderType:             model.Individual,
		Type:                   model.Checking,
		RoutingNumber:          "123",
		EncryptedAccountNumber: "151",
		Status:                 model.DepositoryVerified,
		Created:                base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	receiverRepo := &MockRepository{
		Receivers: []*model.Receiver{
			{
				ID:                receiverID,
				Email:             "foo@moov.io",
				DefaultDepository: id.Depository(base.ID()),
				Status:            model.ReceiverVerified,
				Metadata:          "other",
				Created:           base.NewTime(now),
				Updated:           base.NewTime(now),
			},
		},
	}

	router := mux.NewRouter()
	AddReceiverRoutes(log.NewNopLogger(), router, nil, depRepo, receiverRepo)

	body := fmt.Sprintf(`{"defaultDepository": "%s", "metadata": "other data"}`, dep.ID)

	req := httptest.NewRequest("PATCH", fmt.Sprintf("/receivers/%s", receiverID), strings.NewReader(body))
	req.Header.Set("x-user-id", userID.String())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("PATCH", fmt.Sprintf("/receivers/%s", receiverID), strings.NewReader(body))
	// make the request again with a different userID and verify it fails
	req.Header.Set("x-user-id", base.ID())

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestReceivers__HTTPUpdateError(t *testing.T) {
	receiverID, userID := model.ReceiverID(base.ID()), base.ID()

	repo := &MockRepository{Err: errors.New("bad error")}

	router := mux.NewRouter()
	AddReceiverRoutes(log.NewNopLogger(), router, nil, nil, repo)

	body := strings.NewReader(`{"defaultDepository": "foo", "metadata": "other data"}`)
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/receivers/%s", receiverID), body)
	req.Header.Set("x-user-id", userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestReceivers__HTTPDelete(t *testing.T) {
	userID, now := base.ID(), time.Now()
	rec := &model.Receiver{
		ID:                model.ReceiverID(base.ID()),
		Email:             "foo@moov.io",
		DefaultDepository: id.Depository(base.ID()),
		Status:            model.ReceiverVerified,
		Metadata:          "other",
		Created:           base.NewTime(now),
		Updated:           base.NewTime(now),
	}
	repo := &MockRepository{
		Receivers: []*model.Receiver{rec},
	}

	router := mux.NewRouter()
	AddReceiverRoutes(log.NewNopLogger(), router, nil, nil, repo)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/receivers/%s", rec.ID), nil)
	req.Header.Set("x-user-id", userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

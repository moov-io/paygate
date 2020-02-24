// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

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
	client "github.com/moov-io/paygate/client"
	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestDepositoryJSON(t *testing.T) {
	keeper := secrets.TestStringKeeper(t)
	num, _ := keeper.EncryptString("123")
	bs, err := json.MarshalIndent(model.Depository{
		ID:                     id.Depository(base.ID()),
		BankName:               "moov, inc",
		Holder:                 "Jane Smith",
		HolderType:             model.Individual,
		Type:                   model.Checking,
		RoutingNumber:          "987654320",
		EncryptedAccountNumber: num,
		Status:                 model.DepositoryVerified,
		Metadata:               "extra",
		Keeper:                 keeper,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("  %s", string(bs))

	req := depositoryRequest{
		keeper: keeper,
	}
	bs = []byte(`{
  "bankName": "moov, inc",
  "holder": "john doe",
  "holderType": "business",
  "type": "savings",
  "routingNumber": "123456789",
  "accountNumber": "63531",
  "metadata": "extra"
}`)
	if err := json.NewDecoder(bytes.NewReader(bs)).Decode(&req); err != nil {
		t.Fatal(err)
	}

	t.Logf("req=%#v", req)
}

func TestDepositories__depositoryRequest(t *testing.T) {
	req := depositoryRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestDepositories__read(t *testing.T) {
	body := strings.NewReader(`{
"bankName":    "test",
"holder":      "me",
"holderType":  "Individual",
"type": "Checking",
"metadata": "extra data",
"routingNumber": "123456789",
"accountNumber": "123"
}`)
	keeper := secrets.TestStringKeeper(t)
	req, err := readDepositoryRequest(&http.Request{
		Body: ioutil.NopCloser(body),
	}, keeper)
	if err != nil {
		t.Fatal(err)
	}
	if req.bankName != "test" {
		t.Error(req.bankName)
	}
	if req.holder != "me" {
		t.Error(req.holder)
	}
	if req.holderType != model.Individual {
		t.Error(req.holderType)
	}
	if req.accountType != model.Checking {
		t.Error(req.accountType)
	}
	if req.routingNumber != "123456789" {
		t.Error(req.routingNumber)
	}
	if num, err := keeper.DecryptString(req.accountNumber); err != nil {
		t.Fatal(err)
	} else {
		if num != "123" {
			t.Errorf("num=%s", req.accountNumber)
		}
	}
}

func TestDepositorStatus__json(t *testing.T) {
	ht := model.DepositoryStatus("invalid")
	valid := map[string]model.DepositoryStatus{
		"Verified":   model.DepositoryVerified,
		"unverifieD": model.DepositoryUnverified,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &ht); err != nil {
			t.Error(err.Error())
		}
		if ht != v {
			t.Errorf("got ht=%#v, v=%#v", ht, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, base.ID()))
	if err := json.Unmarshal(in, &ht); err == nil {
		t.Error("expected error")
	}
}

func TestDepositories__HTTPCreate(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	userID := id.User(base.ID())

	accountsClient := &accounts.MockClient{}
	fedClient := &fed.TestClient{}

	keeper := secrets.TestStringKeeper(t)
	repo := NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)

	testODFIAccount := makeTestODFIAccount()

	router := &Router{
		logger:         log.NewNopLogger(),
		odfiAccount:    testODFIAccount,
		accountsClient: accountsClient,
		fedClient:      fedClient,
		depositoryRepo: repo,
		keeper:         keeper,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	body := strings.NewReader(`{
"bankName":    "bank",
"holder":      "holder",
"holderType":  "Individual",
"type": "model.Checking",
"metadata": "extra data",
}`)
	request := httptest.NewRequest("POST", "/depositories", body)
	request.Header.Set("x-user-id", userID.String())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	// Retry with full/valid request
	body = strings.NewReader(`{
"bankName":    "bank",
"holder":      "holder",
"holderType":  "Individual",
"type": "Checking",
"metadata": "extra data",
"routingNumber": "121421212",
"accountNumber": "1321"
}`)
	request = httptest.NewRequest("POST", "/depositories", body)
	request.Header.Set("x-user-id", userID.String())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	w.Flush()

	if w.Code != http.StatusCreated {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	t.Logf(w.Body.String())

	var depository model.Depository
	if err := json.NewDecoder(w.Body).Decode(&depository); err != nil {
		t.Error(err)
	}
	if depository.Status != model.DepositoryUnverified {
		t.Errorf("unexpected status: %s", depository.Status)
	}
}

func TestDepositories__HTTPCreateNoUserID(t *testing.T) {
	repo := &MockRepository{}
	router := &Router{
		logger:         log.NewNopLogger(),
		depositoryRepo: repo,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	body := strings.NewReader(`{"key": "value"}`)
	req := httptest.NewRequest("POST", "/depositories", body)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestDepositories__HTTPUpdate(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	userID, now := id.User(base.ID()), time.Now()
	keeper := secrets.TestStringKeeper(t)

	repo := NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)
	dep := &model.Depository{
		ID:            id.Depository(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    model.Individual,
		Type:          model.Checking,
		RoutingNumber: "121421212",
		Status:        model.DepositoryUnverified,
		Metadata:      "metadata",
		Created:       base.NewTime(now),
		Updated:       base.NewTime(now),
		Keeper:        keeper,
	}
	if err := dep.ReplaceAccountNumber("1234"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}
	if dep, _ := repo.GetUserDepository(dep.ID, userID); dep == nil {
		t.Fatal("nil Depository")
	}

	accountsClient := &accounts.MockClient{}
	testODFIAccount := makeTestODFIAccount()
	testODFIAccount.keeper = keeper

	router := &Router{
		logger:         log.NewNopLogger(),
		odfiAccount:    testODFIAccount,
		accountsClient: accountsClient,
		depositoryRepo: repo,
		keeper:         keeper,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	body := strings.NewReader(`{"accountNumber": "2515219", "bankName": "bar", "holder": "foo", "holderType": "business", "metadata": "updated"}`)
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/depositories/%s", dep.ID), body)
	req.Header.Set("x-user-id", userID.String())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	var depository client.Depository
	if err := json.NewDecoder(w.Body).Decode(&depository); err != nil {
		t.Error(err)
	}
	if !strings.EqualFold(depository.Status, "Unverified") {
		t.Errorf("unexpected status: %s", depository.Status)
	}
	if depository.Metadata != "updated" {
		t.Errorf("unexpected Depository metadata: %s", depository.Metadata)
	}

	// make another request
	body = strings.NewReader(`{"routingNumber": "231380104", "type": "savings"}`)
	req = httptest.NewRequest("PATCH", fmt.Sprintf("/depositories/%s", dep.ID), body)
	req.Header.Set("x-user-id", userID.String())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
	if err := json.NewDecoder(w.Body).Decode(&depository); err != nil {
		t.Error(err)
	}
	if depository.RoutingNumber != "231380104" {
		t.Errorf("depository.RoutingNumber=%s", depository.RoutingNumber)
	}
}

func TestDepositories__HTTPUpdateNoUserID(t *testing.T) {
	repo := &MockRepository{}
	router := &Router{
		logger:         log.NewNopLogger(),
		depositoryRepo: repo,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	body := strings.NewReader(`{"key": "value"}`)
	req := httptest.NewRequest("PATCH", "/depositories/foo", body)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestDepositories__HTTPGet(t *testing.T) {
	userID, now := id.User(base.ID()), time.Now()
	keeper := secrets.TestStringKeeper(t)

	depID := base.ID()
	num, _ := keeper.EncryptString("1234")
	dep := &model.Depository{
		ID:                     id.Depository(depID),
		BankName:               "bank name",
		Holder:                 "holder",
		HolderType:             model.Individual,
		Type:                   model.Checking,
		RoutingNumber:          "121421212",
		EncryptedAccountNumber: num,
		Status:                 model.DepositoryUnverified,
		Metadata:               "metadata",
		Created:                base.NewTime(now),
		Updated:                base.NewTime(now),
		Keeper:                 keeper,
	}
	repo := &MockRepository{
		Depositories: []*model.Depository{dep},
	}

	accountsClient := &accounts.MockClient{}
	testODFIAccount := makeTestODFIAccount()

	router := &Router{
		logger:         log.NewNopLogger(),
		odfiAccount:    testODFIAccount,
		accountsClient: accountsClient,
		depositoryRepo: repo,
		keeper:         keeper,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	req := httptest.NewRequest("GET", fmt.Sprintf("/depositories/%s", dep.ID), nil)
	req.Header.Set("x-user-id", userID.String())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	var depository client.Depository
	if err := json.NewDecoder(w.Body).Decode(&depository); err != nil {
		t.Error(err)
	}
	if depository.ID != depID {
		t.Errorf("unexpected depository: %s", depository.ID)
	}
	if depository.AccountNumber != "1234" {
		t.Errorf("AccountNumber=%s", depository.AccountNumber)
	}
	if !strings.EqualFold(depository.Status, "unverified") {
		t.Errorf("unexpected status: %s", depository.Status)
	}
}

func TestDepositories__HTTPGetNoUserID(t *testing.T) {
	repo := &MockRepository{}
	router := &Router{
		logger:         log.NewNopLogger(),
		depositoryRepo: repo,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	body := strings.NewReader(`{"key": "value"}`)
	req := httptest.NewRequest("GET", "/depositories/foo", body)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestDepositoriesHTTP__delete(t *testing.T) {
	repo := &MockRepository{}
	router := &Router{
		logger:         log.NewNopLogger(),
		depositoryRepo: repo,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	req := httptest.NewRequest("DELETE", "/depositories/foo", nil)
	req.Header.Set("x-user-id", "user")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	// sad path
	repo.Err = errors.New("bad error")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

func TestDepositoriesHTTP__deleteNoUserID(t *testing.T) {
	repo := &MockRepository{}
	router := &Router{
		logger:         log.NewNopLogger(),
		depositoryRepo: repo,
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)

	req := httptest.NewRequest("DELETE", "/depositories/foo", nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

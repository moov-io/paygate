// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

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

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type mockDepositoryRepository struct {
	depositories  []*Depository
	microDeposits []microDeposit
	err           error

	// Updated fields
	status DepositoryStatus
}

func (r *mockDepositoryRepository) getUserDepositories(userId string) ([]*Depository, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.depositories, nil
}

func (r *mockDepositoryRepository) getUserDepository(id DepositoryID, userId string) (*Depository, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(r.depositories) > 0 {
		return r.depositories[0], nil
	}
	return nil, nil
}

func (r *mockDepositoryRepository) upsertUserDepository(userId string, dep *Depository) error {
	return r.err
}

func (r *mockDepositoryRepository) updateDepositoryStatus(id DepositoryID, status DepositoryStatus) error {
	r.status = status
	return r.err
}

func (r *mockDepositoryRepository) deleteUserDepository(id DepositoryID, userId string) error {
	return r.err
}

func (r *mockDepositoryRepository) getMicroDeposits(id DepositoryID, userId string) ([]microDeposit, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.microDeposits, nil
}

func (r *mockDepositoryRepository) initiateMicroDeposits(id DepositoryID, userId string, microDeposit []microDeposit) error {
	return r.err
}

func (r *mockDepositoryRepository) confirmMicroDeposits(id DepositoryID, userId string, amounts []Amount) error {
	return r.err
}

func TestDepositories__depositoryRequest(t *testing.T) {
	req := depositoryRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestDepository__types(t *testing.T) {
	if !DepositoryStatus("").empty() {
		t.Error("expected empty")
	}
}

func TestDepositoriesHolderType__json(t *testing.T) {
	ht := HolderType("invalid")
	valid := map[string]HolderType{
		"indIVIdual": Individual,
		"Business":   Business,
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

func TestDepositories__read(t *testing.T) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(depositoryRequest{
		BankName:      "test",
		Holder:        "me",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123456789",
		AccountNumber: "123",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := readDepositoryRequest(&http.Request{
		Body: ioutil.NopCloser(&buf),
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.BankName != "test" {
		t.Error(req.BankName)
	}
	if req.Holder != "me" {
		t.Error(req.Holder)
	}
	if req.HolderType != Individual {
		t.Error(req.HolderType)
	}
	if req.Type != Checking {
		t.Error(req.Type)
	}
	if req.RoutingNumber != "123456789" {
		t.Error(req.RoutingNumber)
	}
	if req.AccountNumber != "123" {
		t.Error(req.AccountNumber)
	}
}

func TestDepositorStatus__json(t *testing.T) {
	ht := DepositoryStatus("invalid")
	valid := map[string]DepositoryStatus{
		"Verified":   DepositoryVerified,
		"unverifieD": DepositoryUnverified,
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

func TestDepositories__emptyDB(t *testing.T) {
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}

	userId := base.ID()
	if err := r.deleteUserDepository(DepositoryID(base.ID()), userId); err != nil {
		t.Errorf("expected no error, but got %v", err)
	}

	// all depositories for a user
	deps, err := r.getUserDepositories(userId)
	if err != nil {
		t.Error(err)
	}
	if len(deps) != 0 {
		t.Errorf("expected empty, got %v", deps)
	}

	// specific Depository
	dep, err := r.getUserDepository(DepositoryID(base.ID()), userId)
	if err != nil {
		t.Error(err)
	}
	if dep != nil {
		t.Errorf("expected empty, got %v", dep)
	}

	// depository check
	if depositoryIdExists(userId, DepositoryID(base.ID()), r) {
		t.Error("DepositoryId shouldn't exist")
	}
}

func TestDepositories__upsert(t *testing.T) {
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}

	userId := base.ID()

	dep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryVerified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if d, err := r.getUserDepository(dep.ID, userId); err != nil || d != nil {
		t.Errorf("expected empty, d=%v | err=%v", d, err)
	}

	// write, then verify
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Error(err)
	}

	d, err := r.getUserDepository(dep.ID, userId)
	if err != nil {
		t.Error(err)
	}
	if d == nil {
		t.Fatal("expected Depository, got nil")
	}
	if d.ID != dep.ID {
		t.Errorf("d.ID=%q, dep.ID=%q", d.ID, dep.ID)
	}

	// get all for our user
	depositories, err := r.getUserDepositories(userId)
	if err != nil {
		t.Error(err)
	}
	if len(depositories) != 1 {
		t.Errorf("expected one, got %v", depositories)
	}
	if depositories[0].ID != dep.ID {
		t.Errorf("depositories[0].ID=%q, dep.ID=%q", depositories[0].ID, dep.ID)
	}

	// update, verify default depository changed
	bankName := "my new bank"
	dep.BankName = bankName
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Error(err)
	}
	d, err = r.getUserDepository(dep.ID, userId)
	if err != nil {
		t.Error(err)
	}
	if dep.BankName != d.BankName {
		t.Errorf("got %q", d.BankName)
	}
	if d.Status != DepositoryVerified {
		t.Errorf("status: %s", d.Status)
	}
	if !depositoryIdExists(userId, dep.ID, r) {
		t.Error("DepositoryId should exist")
	}
}

func TestDepositories__delete(t *testing.T) {
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}

	userId := base.ID()

	dep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if d, err := r.getUserDepository(dep.ID, userId); err != nil || d != nil {
		t.Errorf("expected empty, d=%v | err=%v", d, err)
	}

	// write
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Error(err)
	}

	// verify
	d, err := r.getUserDepository(dep.ID, userId)
	if err != nil || d == nil {
		t.Errorf("expected depository, d=%v, err=%v", d, err)
	}

	// delete
	if err := r.deleteUserDepository(dep.ID, userId); err != nil {
		t.Error(err)
	}

	// verify tombstoned
	if d, err := r.getUserDepository(dep.ID, userId); err != nil || d != nil {
		t.Errorf("expected empty, d=%v | err=%v", d, err)
	}

	if depositoryIdExists(userId, dep.ID, r) {
		t.Error("DepositoryId shouldn't exist")
	}
}

func TestDepositories__updateDepositoryStatus(t *testing.T) {
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}

	userId := base.ID()
	dep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}

	// write
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Error(err)
	}

	// upsert and read back
	if err := r.updateDepositoryStatus(dep.ID, DepositoryVerified); err != nil {
		t.Fatal(err)
	}
	dep2, err := r.getUserDepository(dep.ID, userId)
	if err != nil {
		t.Fatal(err)
	}
	if dep.ID != dep2.ID {
		t.Errorf("expected=%s got=%s", dep.ID, dep2.ID)
	}
	if dep2.Status != DepositoryVerified {
		t.Errorf("unknown status: %s", dep2.Status)
	}
}

func TestDepositories__markApproved(t *testing.T) {
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}

	userId := base.ID()

	dep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}

	// write
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Error(err)
	}

	// read
	d, err := r.getUserDepository(dep.ID, userId)
	if err != nil || d == nil {
		t.Errorf("expected depository, d=%v, err=%v", d, err)
	}
	if d.Status != DepositoryUnverified {
		t.Errorf("got %v", d.Status)
	}

	// Verify, then re-check
	if err := markDepositoryVerified(r, dep.ID, userId); err != nil {
		t.Fatal(err)
	}

	d, err = r.getUserDepository(dep.ID, userId)
	if err != nil || d == nil {
		t.Errorf("expected depository, d=%v, err=%v", d, err)
	}
	if d.Status != DepositoryVerified {
		t.Errorf("got %v", d.Status)
	}
}

func TestDepositories_OFACMatch(t *testing.T) {
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	depRepo := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}

	userId := "userId"
	request := depositoryRequest{
		BankName:      "my bank",
		Holder:        "john smith",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "121042882", // real routing number
		AccountNumber: "1234",
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/depositories", &body)
	req.Header.Set("x-user-id", userId)

	// happy path, no OFAC match
	fedClient := &testFEDClient{}
	ofacClient := &testOFACClient{}
	createUserDepository(log.NewNopLogger(), fedClient, ofacClient, depRepo)(w, req)
	w.Flush()

	if w.Code != http.StatusCreated {
		t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
	}

	// reset and block via OFAC
	w = httptest.NewRecorder()
	ofacClient = &testOFACClient{
		err: errors.New("blocking"),
	}

	// refill HTTP body
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		t.Fatal(err)
	}
	req.Body = ioutil.NopCloser(&body)

	createUserDepository(log.NewNopLogger(), fedClient, ofacClient, depRepo)(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
	} else {
		if !strings.Contains(w.Body.String(), `ofac: blocking \"john smith\"`) {
			t.Errorf("unknown error: %v", w.Body.String())
		}
	}
}

func TestDepositories__HTTPCreate(t *testing.T) {
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	userId := base.ID()

	fedClient, ofacClient := &testFEDClient{}, &testOFACClient{}
	repo := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}

	router := mux.NewRouter()
	addDepositoryRoutes(log.NewNopLogger(), router, fedClient, ofacClient, repo, nil)

	req := depositoryRequest{
		BankName:   "bank",
		Holder:     "holder",
		HolderType: Individual,
		Type:       Checking,
		// Leave off to test failure
		// RoutingNumber: "121421212",
		// AccountNumber: "1321",
		Metadata: "extra data",
	}

	var body bytes.Buffer
	json.NewEncoder(&body).Encode(req)

	r := httptest.NewRequest("POST", "/depositories", &body)
	r.Header.Set("x-user-id", userId)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	// Retry with full/valid request
	req.RoutingNumber = "121421212"
	req.AccountNumber = "1321"
	json.NewEncoder(&body).Encode(req) // re-encode to bytes.Buffer

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusCreated {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	var depository Depository
	if err := json.NewDecoder(w.Body).Decode(&depository); err != nil {
		t.Error(err)
	}
	if depository.Status != DepositoryUnverified {
		t.Errorf("unexpected status: %s", depository.Status)
	}
}

func TestDepositories__HTTPUpdate(t *testing.T) {
	db, err := database.CreateTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	userId, now := base.ID(), time.Now()

	repo := &sqliteDepositoryRepo{db.DB, log.NewNopLogger()}
	dep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "121421212",
		AccountNumber: "1321",
		Status:        DepositoryUnverified,
		Metadata:      "metadata",
		Created:       base.NewTime(now),
		Updated:       base.NewTime(now),
	}
	if err := repo.upsertUserDepository(userId, dep); err != nil {
		t.Fatal(err)
	}
	if dep, _ := repo.getUserDepository(dep.ID, userId); dep == nil {
		t.Fatal("nil Depository")
	}

	router := mux.NewRouter()
	addDepositoryRoutes(log.NewNopLogger(), router, nil, nil, repo, nil)

	body := strings.NewReader(`{"accountNumber": "251i5219"}`)
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/depositories/%s", dep.ID), body)
	req.Header.Set("x-user-id", userId)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	var depository Depository
	if err := json.NewDecoder(w.Body).Decode(&depository); err != nil {
		t.Error(err)
	}
	if depository.Status != DepositoryUnverified {
		t.Errorf("unexpected status: %s", depository.Status)
	}
}

func TestDepositories__HTTPGet(t *testing.T) {
	userId, now := base.ID(), time.Now()
	dep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "121421212",
		AccountNumber: "1321",
		Status:        DepositoryUnverified,
		Metadata:      "metadata",
		Created:       base.NewTime(now),
		Updated:       base.NewTime(now),
	}
	repo := &mockDepositoryRepository{
		depositories: []*Depository{dep},
	}

	router := mux.NewRouter()
	addDepositoryRoutes(log.NewNopLogger(), router, nil, nil, repo, nil)

	req := httptest.NewRequest("GET", fmt.Sprintf("/depositories/%s", dep.ID), nil)
	req.Header.Set("x-user-id", userId)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	var depository Depository
	if err := json.NewDecoder(w.Body).Decode(&depository); err != nil {
		t.Error(err)
	}
	if depository.ID != dep.ID {
		t.Errorf("unexpected depository: %s", depository.ID)
	}
	if depository.Status != DepositoryUnverified {
		t.Errorf("unexpected status: %s", depository.Status)
	}
}

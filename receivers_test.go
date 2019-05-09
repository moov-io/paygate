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

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type mockReceiverRepository struct {
	receivers []*Receiver
	err       error
}

func (r *mockReceiverRepository) getUserReceivers(userId string) ([]*Receiver, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.receivers, nil
}

func (r *mockReceiverRepository) getUserReceiver(id ReceiverID, userId string) (*Receiver, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(r.receivers) > 0 {
		return r.receivers[0], nil
	}
	return nil, nil
}

func (r *mockReceiverRepository) upsertUserReceiver(userId string, receiver *Receiver) error {
	return r.err
}

func (r *mockReceiverRepository) deleteUserReceiver(id ReceiverID, userId string) error {
	return r.err
}

func TestReceiverStatus__json(t *testing.T) {
	cs := ReceiverStatus("invalid")
	valid := map[string]ReceiverStatus{
		"unverified":  ReceiverUnverified,
		"verIFIed":    ReceiverVerified,
		"SUSPENDED":   ReceiverSuspended,
		"deactivated": ReceiverDeactivated,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &cs); err != nil {
			t.Error(err.Error())
		}
		if cs != v {
			t.Errorf("got cs=%#v, v=%#v", cs, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, nextID()))
	if err := json.Unmarshal(in, &cs); err == nil {
		t.Error("expected error")
	}
}

func TestReceivers__read(t *testing.T) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(receiverRequest{
		Email:             "test@moov.io",
		DefaultDepository: DepositoryID("test"),
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

func TestReceivers__emptyDB(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteReceiverRepo{
		db:  db.db,
		log: log.NewNopLogger(),
	}

	userId := nextID()
	if err := r.deleteUserReceiver(ReceiverID(nextID()), userId); err != nil {
		t.Errorf("expected no error, but got %v", err)
	}

	// all receivers for a user
	receivers, err := r.getUserReceivers(userId)
	if err != nil {
		t.Error(err)
	}
	if len(receivers) != 0 {
		t.Errorf("expected empty, got %v", receivers)
	}

	// specific receiver
	receiver, err := r.getUserReceiver(ReceiverID(nextID()), userId)
	if err != nil {
		t.Error(err)
	}
	if receiver != nil {
		t.Errorf("expected empty, got %v", receiver)
	}
}

func TestReceivers__upsert(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteReceiverRepo{db.db, log.NewNopLogger()}
	userId := nextID()

	receiver := &Receiver{
		ID:                ReceiverID(nextID()),
		Email:             "test@moov.io",
		DefaultDepository: DepositoryID(nextID()),
		Status:            ReceiverVerified,
		Metadata:          "extra data",
		Created:           base.NewTime(time.Now()),
	}
	if c, err := r.getUserReceiver(receiver.ID, userId); err != nil || c != nil {
		t.Errorf("expected empty, c=%v | err=%v", c, err)
	}

	// write, then verify
	if err := r.upsertUserReceiver(userId, receiver); err != nil {
		t.Error(err)
	}

	c, err := r.getUserReceiver(receiver.ID, userId)
	if err != nil {
		t.Error(err)
	}
	if c.ID != receiver.ID {
		t.Errorf("c.ID=%q, receiver.ID=%q", c.ID, receiver.ID)
	}
	if c.Email != receiver.Email {
		t.Errorf("c.Email=%q, receiver.Email=%q", c.Email, receiver.Email)
	}
	if c.DefaultDepository != receiver.DefaultDepository {
		t.Errorf("c.DefaultDepository=%q, receiver.DefaultDepository=%q", c.DefaultDepository, receiver.DefaultDepository)
	}
	if c.Status != receiver.Status {
		t.Errorf("c.Status=%q, receiver.Status=%q", c.Status, receiver.Status)
	}
	if c.Metadata != receiver.Metadata {
		t.Errorf("c.Metadata=%q, receiver.Metadata=%q", c.Metadata, receiver.Metadata)
	}
	if !c.Created.Equal(receiver.Created) {
		t.Errorf("c.Created=%q, receiver.Created=%q", c.Created, receiver.Created)
	}

	// get all for our user
	receivers, err := r.getUserReceivers(userId)
	if err != nil {
		t.Error(err)
	}
	if len(receivers) != 1 {
		t.Errorf("expected one, got %v", receivers)
	}
	if receivers[0].ID != receiver.ID {
		t.Errorf("receivers[0].ID=%q, receiver.ID=%q", receivers[0].ID, receiver.ID)
	}

	// update, verify default depository changed
	depositoryId := DepositoryID(nextID())
	receiver.DefaultDepository = depositoryId
	if err := r.upsertUserReceiver(userId, receiver); err != nil {
		t.Error(err)
	}
	if receiver.DefaultDepository != depositoryId {
		t.Errorf("got %q", receiver.DefaultDepository)
	}
}

// TestReceivers__upsert2 uperts a Receiver twice, which
// will evaluate the whole method.
func TestReceivers__upsert2(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteReceiverRepo{db.db, log.NewNopLogger()}
	userId := nextID()

	receiver := &Receiver{
		ID:                ReceiverID(nextID()),
		Email:             "test@moov.io",
		DefaultDepository: DepositoryID(nextID()),
		Status:            ReceiverUnverified,
		Metadata:          "extra data",
		Created:           base.NewTime(time.Now()),
	}
	if c, err := r.getUserReceiver(receiver.ID, userId); err != nil || c != nil {
		t.Errorf("expected empty, c=%v | err=%v", c, err)
	}

	// initial create, then update
	if err := r.upsertUserReceiver(userId, receiver); err != nil {
		t.Error(err)
	}

	receiver.DefaultDepository = DepositoryID(nextID())
	receiver.Status = ReceiverVerified
	if err := r.upsertUserReceiver(userId, receiver); err != nil {
		t.Error(err)
	}

	c, err := r.getUserReceiver(receiver.ID, userId)
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultDepository == receiver.DefaultDepository {
		t.Error("DefaultDepository should have been updated")
	}
	if c.Status == receiver.Status {
		t.Error("Status should have been updated")
	}
}

func TestReceivers__delete(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteReceiverRepo{db.db, log.NewNopLogger()}
	userId := nextID()

	receiver := &Receiver{
		ID:                ReceiverID(nextID()),
		Email:             "test@moov.io",
		DefaultDepository: DepositoryID(nextID()),
		Status:            ReceiverVerified,
		Metadata:          "extra data",
		Created:           base.NewTime(time.Now()),
	}
	if c, err := r.getUserReceiver(receiver.ID, userId); err != nil || c != nil {
		t.Errorf("expected empty, c=%v | err=%v", c, err)
	}

	// write
	if err := r.upsertUserReceiver(userId, receiver); err != nil {
		t.Error(err)
	}

	// verify
	c, err := r.getUserReceiver(receiver.ID, userId)
	if err != nil || c == nil {
		t.Errorf("expected receiver, c=%v, err=%v", c, err)
	}

	// delete
	if err := r.deleteUserReceiver(receiver.ID, userId); err != nil {
		t.Error(err)
	}

	// verify tombstoned
	if c, err := r.getUserReceiver(receiver.ID, userId); err != nil || c != nil {
		t.Errorf("expected empty, c=%v | err=%v", c, err)
	}
}

func TestReceivers_OFACMatch(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	receiverRepo := &sqliteReceiverRepo{db.db, log.NewNopLogger()}
	depRepo := &sqliteDepositoryRepo{db.db, log.NewNopLogger()}

	// Write Depository to repo
	userId := nextID()
	dep := &Depository{
		ID:            DepositoryID(nextID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
	}
	if err := depRepo.upsertUserDepository(userId, dep); err != nil {
		t.Fatal(err)
	}

	rawBody := fmt.Sprintf(`{"defaultDepository": "%s", "email": "test@example.com", "metadata": "Jane Doe"}`, dep.ID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/receivers", strings.NewReader(rawBody))
	req.Header.Set("x-user-id", userId)

	// happy path, no OFAC match
	client := &testOFACClient{}
	createUserReceiver(log.NewNopLogger(), client, receiverRepo, depRepo)(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
	}

	// reset and block via OFAC
	w = httptest.NewRecorder()
	client = &testOFACClient{
		err: errors.New("blocking"),
	}
	req.Body = ioutil.NopCloser(strings.NewReader(rawBody))
	createUserReceiver(log.NewNopLogger(), client, receiverRepo, depRepo)(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
	} else {
		if !strings.Contains(w.Body.String(), `ofac: blocking \"Jane Doe\"`) {
			t.Errorf("unknown error: %v", w.Body.String())
		}
	}
}

func TestReceivers__HTTPGet(t *testing.T) {
	userId, now := base.ID(), time.Now()
	rec := &Receiver{
		ID:                ReceiverID(base.ID()),
		Email:             "foo@moov.io",
		DefaultDepository: DepositoryID(base.ID()),
		Status:            ReceiverVerified,
		Metadata:          "other",
		Created:           base.NewTime(now),
		Updated:           base.NewTime(now),
	}
	repo := &mockReceiverRepository{
		receivers: []*Receiver{rec},
	}

	router := mux.NewRouter()
	addReceiverRoutes(log.NewNopLogger(), router, nil, repo, nil)

	req := httptest.NewRequest("GET", fmt.Sprintf("/receivers/%s", rec.ID), nil)
	req.Header.Set("x-user-id", userId)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}

	var receiver Receiver
	if err := json.NewDecoder(w.Body).Decode(&receiver); err != nil {
		t.Error(err)
	}
	if receiver.ID != rec.ID {
		t.Errorf("unexpected receiver: %s", receiver.ID)
	}
}

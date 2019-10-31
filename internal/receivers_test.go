// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"database/sql"
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
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type mockReceiverRepository struct {
	receivers []*Receiver
	err       error
}

func (r *mockReceiverRepository) getUserReceivers(userID string) ([]*Receiver, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.receivers, nil
}

func (r *mockReceiverRepository) getUserReceiver(id ReceiverID, userID string) (*Receiver, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(r.receivers) > 0 {
		return r.receivers[0], nil
	}
	return nil, nil
}

func (r *mockReceiverRepository) upsertUserReceiver(userID string, receiver *Receiver) error {
	return r.err
}

func (r *mockReceiverRepository) deleteUserReceiver(id ReceiverID, userID string) error {
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
	in := []byte(fmt.Sprintf(`"%v"`, base.ID()))
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
	t.Parallel()

	check := func(t *testing.T, repo receiverRepository) {
		userID := base.ID()
		if err := repo.deleteUserReceiver(ReceiverID(base.ID()), userID); err != nil {
			t.Errorf("expected no error, but got %v", err)
		}

		// all receivers for a user
		receivers, err := repo.getUserReceivers(userID)
		if err != nil {
			t.Error(err)
		}
		if len(receivers) != 0 {
			t.Errorf("expected empty, got %v", receivers)
		}

		// specific receiver
		receiver, err := repo.getUserReceiver(ReceiverID(base.ID()), userID)
		if err != nil {
			t.Error(err)
		}
		if receiver != nil {
			t.Errorf("expected empty, got %v", receiver)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLReceiverRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLReceiverRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestReceivers__upsert(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo receiverRepository) {
		userID := base.ID()
		receiver := &Receiver{
			ID:                ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: DepositoryID(base.ID()),
			Status:            ReceiverVerified,
			Metadata:          "extra data",
			Created:           base.NewTime(time.Now()),
		}
		if c, err := repo.getUserReceiver(receiver.ID, userID); err != nil || c != nil {
			t.Errorf("expected empty, c=%v | err=%v", c, err)
		}

		// write, then verify
		if err := repo.upsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		c, err := repo.getUserReceiver(receiver.ID, userID)
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
		receivers, err := repo.getUserReceivers(userID)
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
		depositoryId := DepositoryID(base.ID())
		receiver.DefaultDepository = depositoryId
		if err := repo.upsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}
		if receiver.DefaultDepository != depositoryId {
			t.Errorf("got %q", receiver.DefaultDepository)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLReceiverRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLReceiverRepo{mysqlDB.DB, log.NewNopLogger()})
}

// TestReceivers__upsert2 uperts a Receiver twice, which
// will evaluate the whole method.
func TestReceivers__upsert2(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo receiverRepository) {
		customerID, userID := base.ID(), base.ID()
		defaultDepository, status := base.ID(), ReceiverUnverified
		receiver := &Receiver{
			ID:                ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: DepositoryID(defaultDepository),
			Status:            status,
			CustomerID:        customerID,
			Metadata:          "extra data",
			Created:           base.NewTime(time.Now()),
		}
		if c, err := repo.getUserReceiver(receiver.ID, userID); err != nil || c != nil {
			t.Errorf("expected empty, c=%v | err=%v", c, err)
		}

		// initial create, then update
		if err := repo.upsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		receiver.DefaultDepository = DepositoryID(base.ID())
		receiver.Status = ReceiverVerified
		if err := repo.upsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		r, err := repo.getUserReceiver(receiver.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if DepositoryID(defaultDepository) == r.DefaultDepository {
			t.Errorf("DefaultDepository should have been updated (original:%s) (current:%s)", defaultDepository, r.DefaultDepository)
		}
		if status == r.Status {
			t.Errorf("Status should have been updated (original:%s) (current:%s)", status, r.Status)
		}

		if r.CustomerID != customerID {
			t.Errorf("receiver CustomerID=%s", r.CustomerID)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLReceiverRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLReceiverRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestReceivers__delete(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo receiverRepository) {
		userID := base.ID()
		receiver := &Receiver{
			ID:                ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: DepositoryID(base.ID()),
			Status:            ReceiverVerified,
			Metadata:          "extra data",
			Created:           base.NewTime(time.Now()),
		}
		if c, err := repo.getUserReceiver(receiver.ID, userID); err != nil || c != nil {
			t.Errorf("expected empty, c=%v | err=%v", c, err)
		}

		// write
		if err := repo.upsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		// verify
		c, err := repo.getUserReceiver(receiver.ID, userID)
		if err != nil || c == nil {
			t.Errorf("expected receiver, c=%v, err=%v", c, err)
		}

		// delete
		if err := repo.deleteUserReceiver(receiver.ID, userID); err != nil {
			t.Error(err)
		}

		// verify tombstoned
		if c, err := repo.getUserReceiver(receiver.ID, userID); err != nil || c != nil {
			t.Errorf("expected empty, c=%v | err=%v", c, err)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLReceiverRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLReceiverRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestReceivers_CustomersError(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, db *sql.DB) {
		receiverRepo := &SQLReceiverRepo{db, log.NewNopLogger()}
		depRepo := &SQLDepositoryRepo{db, log.NewNopLogger()}

		// Write Depository to repo
		userID := base.ID()
		dep := &Depository{
			ID:            DepositoryID(base.ID()),
			BankName:      "bank name",
			Holder:        "holder",
			HolderType:    Individual,
			Type:          Checking,
			RoutingNumber: "123",
			AccountNumber: "151",
			Status:        DepositoryUnverified,
		}
		if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
			t.Fatal(err)
		}

		rawBody := fmt.Sprintf(`{"defaultDepository": "%s", "email": "test@example.com", "metadata": "Jane Doe"}`, dep.ID)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/receivers", strings.NewReader(rawBody))
		req.Header.Set("x-user-id", userID)

		// happy path, no Customers match
		createUserReceiver(log.NewNopLogger(), nil, depRepo, receiverRepo)(w, req)
		w.Flush()

		if w.Code != http.StatusOK {
			t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
		}

		// reset and block
		w = httptest.NewRecorder()
		client := &customers.TestClient{
			Err: errors.New("bad error"),
		}
		req.Body = ioutil.NopCloser(strings.NewReader(rawBody))
		createUserReceiver(log.NewNopLogger(), client, depRepo, receiverRepo)(w, req)
		w.Flush()

		if w.Code != http.StatusBadRequest {
			t.Errorf("bogus status code: %d: %v", w.Code, w.Body.String())
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, sqliteDB.DB)

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, mysqlDB.DB)
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
	AddReceiverRoutes(log.NewNopLogger(), router, nil, nil, repo)

	req := httptest.NewRequest("GET", fmt.Sprintf("/receivers/%s", rec.ID), nil)
	req.Header.Set("x-user-id", userID)

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

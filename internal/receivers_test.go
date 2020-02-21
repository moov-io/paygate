// Copyright 2020 The Moov Authors
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
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type mockReceiverRepository struct {
	receivers []*Receiver
	err       error
}

func (r *mockReceiverRepository) getUserReceivers(userID id.User) ([]*Receiver, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.receivers, nil
}

func (r *mockReceiverRepository) getUserReceiver(id ReceiverID, userID id.User) (*Receiver, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(r.receivers) > 0 {
		return r.receivers[0], nil
	}
	return nil, nil
}

func (r *mockReceiverRepository) updateReceiverStatus(id ReceiverID, status ReceiverStatus) error {
	return r.err
}

func (r *mockReceiverRepository) upsertUserReceiver(userID id.User, receiver *Receiver) error {
	return r.err
}

func (r *mockReceiverRepository) deleteUserReceiver(id ReceiverID, userID id.User) error {
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

func TestReceivers__emptyDB(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo receiverRepository) {
		userID := id.User(base.ID())
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
		userID := id.User(base.ID())
		receiver := &Receiver{
			ID:                ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: id.Depository(base.ID()),
			Status:            ReceiverVerified,
			Metadata:          "extra data",
			Created:           base.NewTime(time.Now().Truncate(1 * time.Second)),
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
		depositoryId := id.Depository(base.ID())
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
		customerID, userID := base.ID(), id.User(base.ID())
		defaultDepository, status := base.ID(), ReceiverUnverified
		receiver := &Receiver{
			ID:                ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: id.Depository(defaultDepository),
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

		receiver.DefaultDepository = id.Depository(base.ID())
		receiver.Status = ReceiverVerified
		if err := repo.upsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		r, err := repo.getUserReceiver(receiver.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if id.Depository(defaultDepository) == r.DefaultDepository {
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

func TestReceivers__updateReceiverStatus(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo receiverRepository) {
		userID := id.User(base.ID())
		receiver := &Receiver{
			ID:                ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: id.Depository(base.ID()),
			Status:            ReceiverVerified,
			Metadata:          "extra data",
			Created:           base.NewTime(time.Now()),
		}
		if err := repo.upsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		// verify before our update
		r, err := repo.getUserReceiver(receiver.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != ReceiverVerified {
			t.Errorf("r.Status=%v", r.Status)
		}

		// update and verify
		if err := repo.updateReceiverStatus(receiver.ID, ReceiverSuspended); err != nil {
			t.Fatal(err)
		}
		r, err = repo.getUserReceiver(receiver.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != ReceiverSuspended {
			t.Errorf("r.Status=%v", r.Status)
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
		userID := id.User(base.ID())
		receiver := &Receiver{
			ID:                ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: id.Depository(base.ID()),
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

	keeper := secrets.TestStringKeeper(t)
	check := func(t *testing.T, db *sql.DB) {
		receiverRepo := &SQLReceiverRepo{db, log.NewNopLogger()}
		depRepo := NewDepositoryRepo(log.NewNopLogger(), db, keeper)

		// Write Depository to repo
		userID := id.User(base.ID())
		dep := &model.Depository{
			ID:                     id.Depository(base.ID()),
			BankName:               "bank name",
			Holder:                 "holder",
			HolderType:             model.Individual,
			Type:                   model.Checking,
			RoutingNumber:          "123",
			EncryptedAccountNumber: "151",
			Status:                 model.DepositoryUnverified,
		}
		if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
			t.Fatal(err)
		}

		rawBody := fmt.Sprintf(`{"defaultDepository": "%s", "email": "test@example.com", "metadata": "Jane Doe"}`, dep.ID)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/receivers", strings.NewReader(rawBody))
		req.Header.Set("x-user-id", userID.String())

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
		DefaultDepository: id.Depository(base.ID()),
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

func TestReceivers__HTTPGetNoUserID(t *testing.T) {
	repo := &mockReceiverRepository{}

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
	receiverID, userID := ReceiverID(base.ID()), id.User(base.ID())

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	keeper := secrets.TestStringKeeper(t)
	depRepo := NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper)
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

	receiverRepo := &mockReceiverRepository{
		receivers: []*Receiver{
			{
				ID:                receiverID,
				Email:             "foo@moov.io",
				DefaultDepository: id.Depository(base.ID()),
				Status:            ReceiverVerified,
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
	receiverID, userID := ReceiverID(base.ID()), base.ID()

	repo := &mockReceiverRepository{err: errors.New("bad error")}

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
	rec := &Receiver{
		ID:                ReceiverID(base.ID()),
		Email:             "foo@moov.io",
		DefaultDepository: id.Depository(base.ID()),
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

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/receivers/%s", rec.ID), nil)
	req.Header.Set("x-user-id", userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP status: %d: %s", w.Code, w.Body.String())
	}
}

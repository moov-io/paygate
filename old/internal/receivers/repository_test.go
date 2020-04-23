// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package receivers

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestReceivers__emptyDB(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
		userID := id.User(base.ID())
		if err := repo.deleteUserReceiver(model.ReceiverID(base.ID()), userID); err != nil {
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
		receiver, err := repo.GetUserReceiver(model.ReceiverID(base.ID()), userID)
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

	check := func(t *testing.T, repo Repository) {
		userID := id.User(base.ID())
		receiver := &model.Receiver{
			ID:                model.ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: id.Depository(base.ID()),
			Status:            model.ReceiverVerified,
			Metadata:          "extra data",
			Created:           base.NewTime(time.Now().Truncate(1 * time.Second)),
		}
		if c, err := repo.GetUserReceiver(receiver.ID, userID); err != nil || c != nil {
			t.Errorf("expected empty, c=%v | err=%v", c, err)
		}

		// write, then verify
		if err := repo.UpsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		c, err := repo.GetUserReceiver(receiver.ID, userID)
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
		if err := repo.UpsertUserReceiver(userID, receiver); err != nil {
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

	check := func(t *testing.T, repo Repository) {
		customerID, userID := base.ID(), id.User(base.ID())
		defaultDepository, status := base.ID(), model.ReceiverUnverified
		receiver := &model.Receiver{
			ID:                model.ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: id.Depository(defaultDepository),
			Status:            status,
			CustomerID:        customerID,
			Metadata:          "extra data",
			Created:           base.NewTime(time.Now()),
		}
		if c, err := repo.GetUserReceiver(receiver.ID, userID); err != nil || c != nil {
			t.Errorf("expected empty, c=%v | err=%v", c, err)
		}

		// initial create, then update
		if err := repo.UpsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		receiver.DefaultDepository = id.Depository(base.ID())
		receiver.Status = model.ReceiverVerified
		if err := repo.UpsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		r, err := repo.GetUserReceiver(receiver.ID, userID)
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

func TestReceivers__UpdateReceiverStatus(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo Repository) {
		userID := id.User(base.ID())
		receiver := &model.Receiver{
			ID:                model.ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: id.Depository(base.ID()),
			Status:            model.ReceiverVerified,
			Metadata:          "extra data",
			Created:           base.NewTime(time.Now()),
		}
		if err := repo.UpsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		// verify before our update
		r, err := repo.GetUserReceiver(receiver.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != model.ReceiverVerified {
			t.Errorf("r.Status=%v", r.Status)
		}

		// update and verify
		if err := repo.UpdateReceiverStatus(receiver.ID, model.ReceiverSuspended); err != nil {
			t.Fatal(err)
		}
		r, err = repo.GetUserReceiver(receiver.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != model.ReceiverSuspended {
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

	check := func(t *testing.T, repo Repository) {
		userID := id.User(base.ID())
		receiver := &model.Receiver{
			ID:                model.ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: id.Depository(base.ID()),
			Status:            model.ReceiverVerified,
			Metadata:          "extra data",
			Created:           base.NewTime(time.Now()),
		}
		if c, err := repo.GetUserReceiver(receiver.ID, userID); err != nil || c != nil {
			t.Errorf("expected empty, c=%v | err=%v", c, err)
		}

		// write
		if err := repo.UpsertUserReceiver(userID, receiver); err != nil {
			t.Error(err)
		}

		// verify
		c, err := repo.GetUserReceiver(receiver.ID, userID)
		if err != nil || c == nil {
			t.Errorf("expected receiver, c=%v, err=%v", c, err)
		}

		// delete
		if err := repo.deleteUserReceiver(receiver.ID, userID); err != nil {
			t.Error(err)
		}

		// verify tombstoned
		if c, err := repo.GetUserReceiver(receiver.ID, userID); err != nil || c != nil {
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
		depRepo := depository.NewDepositoryRepo(log.NewNopLogger(), db, keeper)

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

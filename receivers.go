// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type ReceiverID string

// Receiver objects are organizations or people who receive an ACH Transfer from an Originator account.
//
// The API allows you to create, delete, and update your originators.
// You can retrieve individual originators as well as a list of all your originators. (Batch Header)
type Receiver struct {
	// ID is a unique string representing this Receiver.
	ID ReceiverID `json:"id"`

	// Email address associated to Receiver
	Email string `json:"email"` // TODO(adam): validate, public suffix list (PSL)

	// DefaultDepository is the Depository associated to this Receiver.
	DefaultDepository DepositoryID `json:"defaultDepository"`

	// Status defines the current state of the Receiver
	Status ReceiverStatus `json:"status"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated base.Time `json:"updated"`
}

func (c *Receiver) missingFields() error {
	if c.ID == "" {
		return errors.New("missing Receiver.ID")
	}
	if c.Email == "" {
		return errors.New("missing Receiver.Email")
	}
	if c.DefaultDepository == "" {
		return errors.New("missing Receiver.DefaultDepository")
	}
	if c.Status == "" {
		return errors.New("missing Receiver.Status")
	}
	return nil
}

// Validate checks the fields of Receiver and returns any validation errors.
func (c *Receiver) validate() error {
	if c == nil {
		return errors.New("nil Receiver")
	}
	if err := c.missingFields(); err != nil {
		return err
	}

	// TODO(adam): validate email
	return c.Status.validate()
}

type ReceiverStatus string

const (
	ReceiverUnverified  ReceiverStatus = "unverified"
	ReceiverVerified    ReceiverStatus = "verified"
	ReceiverSuspended   ReceiverStatus = "suspended"
	ReceiverDeactivated ReceiverStatus = "deactivated"
)

func (cs ReceiverStatus) validate() error {
	switch cs {
	case ReceiverUnverified, ReceiverVerified, ReceiverSuspended, ReceiverDeactivated:
		return nil
	default:
		return fmt.Errorf("ReceiverStatus(%s) is invalid", cs)
	}
}

func (cs *ReceiverStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*cs = ReceiverStatus(strings.ToLower(s))
	if err := cs.validate(); err != nil {
		return err
	}
	return nil
}

type receiverRequest struct {
	Email             string       `json:"email,omitempty"`
	DefaultDepository DepositoryID `json:"defaultDepository,omitempty"`
	Metadata          string       `json:"metadata,omitempty"`
}

func (r receiverRequest) missingFields() error {
	if r.Email == "" {
		return errors.New("missing receiverRequest.Email")
	}
	if r.DefaultDepository.empty() {
		return errors.New("missing receiverRequest.DefaultDepository")
	}
	return nil
}

func addReceiverRoutes(logger log.Logger, r *mux.Router, ofacClient OFACClient, receiverRepo receiverRepository, depositoryRepo depositoryRepository) {
	r.Methods("GET").Path("/receivers").HandlerFunc(getUserReceivers(logger, receiverRepo))
	r.Methods("POST").Path("/receivers").HandlerFunc(createUserReceiver(logger, ofacClient, receiverRepo, depositoryRepo))

	r.Methods("GET").Path("/receivers/{receiverId}").HandlerFunc(getUserReceiver(logger, receiverRepo))
	r.Methods("PATCH").Path("/receivers/{receiverId}").HandlerFunc(updateUserReceiver(logger, receiverRepo))
	r.Methods("DELETE").Path("/receivers/{receiverId}").HandlerFunc(deleteUserReceiver(logger, receiverRepo))
}

func getUserReceivers(logger log.Logger, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		userId := moovhttp.GetUserId(r)
		receivers, err := receiverRepo.getUserReceivers(userId)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(receivers); err != nil {
			internalError(logger, w, err)
			return
		}
	}
}

func readReceiverRequest(r *http.Request) (receiverRequest, error) {
	var req receiverRequest
	bs, err := read(r.Body)
	if err != nil {
		return req, err
	}
	if err := json.Unmarshal(bs, &req); err != nil {
		return req, err
	}
	if err := req.missingFields(); err != nil {
		return req, fmt.Errorf("%v: %v", errMissingRequiredJson, err)
	}
	return req, nil
}

func createUserReceiver(logger log.Logger, ofacClient OFACClient, receiverRepo receiverRepository, depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		req, err := readReceiverRequest(r)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		userId, requestId := moovhttp.GetUserId(r), moovhttp.GetRequestId(r)
		if !depositoryIdExists(userId, req.DefaultDepository, depositoryRepo) {
			moovhttp.Problem(w, fmt.Errorf("depository %s does not exist", req.DefaultDepository))
			return
		}

		// Create our receiver
		receiver := &Receiver{
			ID:                ReceiverID(base.ID()),
			Email:             req.Email,
			DefaultDepository: req.DefaultDepository,
			Status:            ReceiverUnverified,
			Metadata:          req.Metadata,
			Created:           base.NewTime(time.Now()),
		}
		if err := receiver.validate(); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		// Check OFAC for receiver/company data
		if err := rejectViaOFACMatch(logger, ofacClient, receiver.Metadata, userId, requestId); err != nil {
			if logger != nil {
				logger.Log("receivers", err.Error(), "userId", userId)
			}
			moovhttp.Problem(w, err)
			return
		}

		if err := receiverRepo.upsertUserReceiver(userId, receiver); err != nil {
			internalError(logger, w, fmt.Errorf("creating receiver=%q, user_id=%q", receiver.ID, userId))
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(receiver); err != nil {
			internalError(logger, w, err)
			return
		}
	}
}

func getUserReceiver(logger log.Logger, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		id, userId := getReceiverId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver, err := receiverRepo.getUserReceiver(id, userId)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(receiver); err != nil {
			internalError(logger, w, err)
			return
		}
	}
}

func updateUserReceiver(logger log.Logger, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		req, err := readReceiverRequest(r)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		id, userId := getReceiverId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver, err := receiverRepo.getUserReceiver(id, userId)
		if err != nil {
			internalError(logger, w, fmt.Errorf("problem getting receiver=%q, user_id=%q", id, userId))
			return
		}
		if req.DefaultDepository != "" {
			receiver.DefaultDepository = req.DefaultDepository
		}
		if req.Metadata != "" {
			receiver.Metadata = req.Metadata
		}
		receiver.Updated = base.NewTime(time.Now())

		if err := receiver.validate(); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		// Perform update
		if err := receiverRepo.upsertUserReceiver(userId, receiver); err != nil {
			internalError(logger, w, fmt.Errorf("updating receiver=%q, user_id=%q", id, userId))
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(receiver); err != nil {
			internalError(logger, w, err)
			return
		}
	}
}

func deleteUserReceiver(logger log.Logger, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		id, userId := getReceiverId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := receiverRepo.deleteUserReceiver(id, userId); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
	}
}

// getReceiverId extracts the ReceiverID from the incoming request.
func getReceiverId(r *http.Request) ReceiverID {
	v := mux.Vars(r)
	id, ok := v["receiverId"]
	if !ok {
		return ReceiverID("")
	}
	return ReceiverID(id)

}

type receiverRepository interface {
	getUserReceivers(userId string) ([]*Receiver, error)
	getUserReceiver(id ReceiverID, userId string) (*Receiver, error)

	upsertUserReceiver(userId string, receiver *Receiver) error
	deleteUserReceiver(id ReceiverID, userId string) error
}

type sqliteReceiverRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *sqliteReceiverRepo) close() error {
	return r.db.Close()
}

func (r *sqliteReceiverRepo) getUserReceivers(userId string) ([]*Receiver, error) {
	query := `select receiver_id from receivers where user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receiverIds []string
	for rows.Next() {
		var row string
		rows.Scan(&row)
		if row != "" {
			receiverIds = append(receiverIds, row)
		}
	}

	var receivers []*Receiver
	for i := range receiverIds {
		receiver, err := r.getUserReceiver(ReceiverID(receiverIds[i]), userId)
		if err == nil && receiver != nil && receiver.Email != "" {
			receivers = append(receivers, receiver)
		}
	}
	return receivers, rows.Err()
}

func (r *sqliteReceiverRepo) getUserReceiver(id ReceiverID, userId string) (*Receiver, error) {
	query := `select receiver_id, email, default_depository, status, metadata, created_at, last_updated_at
from receivers
where receiver_id = ?
and user_id = ?
and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userId)

	var receiver Receiver
	err = row.Scan(&receiver.ID, &receiver.Email, &receiver.DefaultDepository, &receiver.Status, &receiver.Metadata, &receiver.Created.Time, &receiver.Updated.Time)
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return nil, nil
		}
		return nil, err
	}
	if receiver.ID == "" || receiver.Email == "" {
		return nil, nil // no records found
	}
	return &receiver, nil
}

func (r *sqliteReceiverRepo) upsertUserReceiver(userId string, receiver *Receiver) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	now := time.Now()
	if receiver.Created.IsZero() {
		receiver.Created = base.NewTime(now)
		receiver.Updated = base.NewTime(now)
	}

	query := `insert into receivers (receiver_id, user_id, email, default_depository, status, metadata, created_at, last_updated_at) values (?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("upsertUserReceiver: prepare err=%v: rollback=%v", err, tx.Rollback())
	}

	var (
		created time.Time
		updated time.Time
	)
	res, err := stmt.Exec(receiver.ID, userId, receiver.Email, receiver.DefaultDepository, receiver.Status, receiver.Metadata, &created, &updated)
	stmt.Close()
	if err != nil && !database.UniqueViolation(err) {
		return fmt.Errorf("problem upserting receiver=%q, userId=%q error=%v rollback=%v", receiver.ID, userId, err, tx.Rollback())
	}
	receiver.Created = base.NewTime(created)
	receiver.Updated = base.NewTime(updated)

	// Check and skip ahead if the insert failed (to database.UniqueViolation)
	if res == nil {
		goto update
	}
	if n, _ := res.RowsAffected(); n == 0 {
		goto update
	} else {
		return tx.Commit() // Depository was inserted, so cleanup and exit
	}
	// We should rollback in the event of an unexpected problem. It's not possible to check (res != nil) and
	// call res.RowsAffected() in the same 'if' statement, so we needed multiple.
	return fmt.Errorf("upsertUserReceiver: rollback=%v", tx.Rollback())
update:
	query = `update receivers
set email = ?, default_depository = ?, status = ?, metadata = ?, last_updated_at = ?
where receiver_id = ? and user_id = ? and deleted_at is null`
	stmt, err = tx.Prepare(query)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(receiver.Email, receiver.DefaultDepository, receiver.Status, receiver.Metadata, now, receiver.ID, userId)
	stmt.Close()
	if err != nil {
		return fmt.Errorf("upsertUserReceiver: exec error=%v rollback=%v", err, tx.Rollback())
	}
	return tx.Commit()
}

func (r *sqliteReceiverRepo) deleteUserReceiver(id ReceiverID, userId string) error {
	// TODO(adam): Should this just change the status to Deactivated?
	query := `update receivers set deleted_at = ? where receiver_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(time.Now(), id, userId); err != nil {
		return fmt.Errorf("error deleting receiver_id=%q, user_id=%q: %v", id, userId, err)
	}
	return nil
}

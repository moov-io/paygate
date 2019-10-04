// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/ofac"

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
	Email string `json:"email"`

	// DefaultDepository is the Depository associated to this Receiver.
	DefaultDepository DepositoryID `json:"defaultDepository"`

	// Status defines the current state of the Receiver
	Status ReceiverStatus `json:"status"`
	// TODO(adam): how does this status change? micro-deposit? email? both?

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

func AddReceiverRoutes(logger log.Logger, r *mux.Router, ofacClient ofac.Client, receiverRepo receiverRepository, depositoryRepo DepositoryRepository) {
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

		userID := moovhttp.GetUserID(r)
		receivers, err := receiverRepo.getUserReceivers(userID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(receivers)
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

// parseAndValidateEmail attempts to parse an email address and validate the domain name.
// TODO(adam): call net.DialTimeout (with on/off config) on the domain name?
func parseAndValidateEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return "", fmt.Errorf("error parsing '%s': %v", raw, err)
	}
	return addr.Address, nil
}

func createUserReceiver(logger log.Logger, ofacClient ofac.Client, receiverRepo receiverRepository, depositoryRepo DepositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		userID, requestID := moovhttp.GetUserID(r), moovhttp.GetRequestID(r)

		req, err := readReceiverRequest(r)
		if err != nil {
			logger.Log("receivers", fmt.Errorf("error reading receiverRequest: %v", err), "requestID", requestID)
			moovhttp.Problem(w, err)
			return
		}

		if !depositoryIdExists(userID, req.DefaultDepository, depositoryRepo) {
			err := fmt.Errorf("depository %s does not exist", req.DefaultDepository)
			logger.Log("receivers", fmt.Errorf("error finding Depository: %v", err), "requestID", requestID)
			moovhttp.Problem(w, err)
			return
		}
		email, err := parseAndValidateEmail(req.Email)
		if err != nil {
			logger.Log("receivers", fmt.Sprintf("unable to validate receiver email: %v", err), "requestID", requestID)
			moovhttp.Problem(w, err)
			return
		}

		// Create our receiver
		receiver := &Receiver{
			ID:                ReceiverID(base.ID()),
			Email:             email,
			DefaultDepository: req.DefaultDepository,
			Status:            ReceiverUnverified,
			Metadata:          req.Metadata,
			Created:           base.NewTime(time.Now()),
		}
		if err := receiver.validate(); err != nil {
			logger.Log("receivers", fmt.Errorf("error validating Receiver: %v", err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		// Check OFAC for receiver/company data
		if err := ofac.RejectViaMatch(logger, ofacClient, receiver.Metadata, userID, requestID); err != nil {
			logger.Log("receivers", fmt.Errorf("error with OFAC call: %v", err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		if err := receiverRepo.upsertUserReceiver(userID, receiver); err != nil {
			err = fmt.Errorf("creating receiver=%q, user_id=%q", receiver.ID, userID)
			logger.Log("receivers", fmt.Errorf("error inserting Receiver: %v", err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(receiver)
	}
}

func getUserReceiver(logger log.Logger, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		id, userID := getReceiverID(r), moovhttp.GetUserID(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver, err := receiverRepo.getUserReceiver(id, userID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(receiver)
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

		id, userID := getReceiverID(r), moovhttp.GetUserID(r)
		requestID := moovhttp.GetRequestID(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver, err := receiverRepo.getUserReceiver(id, userID)
		if err != nil {
			logger.Log("originators", fmt.Sprintf("problem getting receiver='%s': %v", id, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
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
		if err := receiverRepo.upsertUserReceiver(userID, receiver); err != nil {
			logger.Log("originators", fmt.Sprintf("problem upserting receiver=%s: %v", receiver.ID, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(receiver)
	}
}

func deleteUserReceiver(logger log.Logger, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		id, userID := getReceiverID(r), moovhttp.GetUserID(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := receiverRepo.deleteUserReceiver(id, userID); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
	}
}

// getReceiverID extracts the ReceiverID from the incoming request.
func getReceiverID(r *http.Request) ReceiverID {
	v := mux.Vars(r)
	id, ok := v["receiverId"]
	if !ok {
		return ReceiverID("")
	}
	return ReceiverID(id)
}

type receiverRepository interface {
	getUserReceivers(userID string) ([]*Receiver, error)
	getUserReceiver(id ReceiverID, userID string) (*Receiver, error)

	upsertUserReceiver(userID string, receiver *Receiver) error
	deleteUserReceiver(id ReceiverID, userID string) error
}

func NewReceiverRepo(logger log.Logger, db *sql.DB) *SQLReceiverRepo {
	return &SQLReceiverRepo{log: logger, db: db}
}

type SQLReceiverRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *SQLReceiverRepo) Close() error {
	return r.db.Close()
}

func (r *SQLReceiverRepo) getUserReceivers(userID string) ([]*Receiver, error) {
	query := `select receiver_id from receivers where user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receiverIDs []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return nil, fmt.Errorf("getUserReceivers scan: %v", err)
		}
		if row != "" {
			receiverIDs = append(receiverIDs, row)
		}
	}

	var receivers []*Receiver
	for i := range receiverIDs {
		receiver, err := r.getUserReceiver(ReceiverID(receiverIDs[i]), userID)
		if err == nil && receiver != nil && receiver.Email != "" {
			receivers = append(receivers, receiver)
		}
	}
	return receivers, rows.Err()
}

func (r *SQLReceiverRepo) getUserReceiver(id ReceiverID, userID string) (*Receiver, error) {
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

	row := stmt.QueryRow(id, userID)

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

func (r *SQLReceiverRepo) upsertUserReceiver(userID string, receiver *Receiver) error {
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
	res, err := stmt.Exec(receiver.ID, userID, receiver.Email, receiver.DefaultDepository, receiver.Status, receiver.Metadata, &created, &updated)
	stmt.Close()
	if err != nil && !database.UniqueViolation(err) {
		return fmt.Errorf("problem upserting receiver=%q, userID=%q error=%v rollback=%v", receiver.ID, userID, err, tx.Rollback())
	}
	receiver.Created = base.NewTime(created)
	receiver.Updated = base.NewTime(updated)

	// Check and skip ahead if the insert failed (to database.UniqueViolation)
	if res != nil {
		if n, _ := res.RowsAffected(); n != 0 {
			return tx.Commit() // Receiver was inserted, so cleanup and exit
		}
	}
	query = `update receivers
set email = ?, default_depository = ?, status = ?, metadata = ?, last_updated_at = ?
where receiver_id = ? and user_id = ? and deleted_at is null`
	stmt, err = tx.Prepare(query)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(receiver.Email, receiver.DefaultDepository, receiver.Status, receiver.Metadata, now, receiver.ID, userID)
	stmt.Close()
	if err != nil {
		return fmt.Errorf("upsertUserReceiver: exec error=%v rollback=%v", err, tx.Rollback())
	}
	return tx.Commit()
}

func (r *SQLReceiverRepo) deleteUserReceiver(id ReceiverID, userID string) error {
	// TODO(adam): Should this just change the status to Deactivated?
	query := `update receivers set deleted_at = ? where receiver_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(time.Now(), id, userID); err != nil {
		return fmt.Errorf("error deleting receiver_id=%q, user_id=%q: %v", id, userID, err)
	}
	return nil
}

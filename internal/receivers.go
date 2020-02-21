// Copyright 2020 The Moov Authors
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
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/pkg/id"

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
	DefaultDepository id.Depository `json:"defaultDepository"`

	// Status defines the current state of the Receiver
	// TODO(adam): how does this status change? micro-deposit? email? both?
	Status ReceiverStatus `json:"status"`

	// BirthDate is an optional value required for Know Your Customer (KYC) validation of this Originator
	BirthDate time.Time `json:"birthDate,omitempty"`

	// Address is an optional object required for Know Your Customer (KYC) validation of this Originator
	Address *model.Address `json:"address,omitempty"`

	// CustomerID is a unique ID that from Moov's Customers service for this Originator
	CustomerID string `json:"customerId"`

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
	Email             string         `json:"email,omitempty"`
	DefaultDepository id.Depository  `json:"defaultDepository,omitempty"`
	BirthDate         time.Time      `json:"birthDate,omitempty"`
	Address           *model.Address `json:"address,omitempty"`
	Metadata          string         `json:"metadata,omitempty"`
}

func (r receiverRequest) missingFields() error {
	if r.Email == "" {
		return errors.New("missing receiverRequest.Email")
	}
	if r.DefaultDepository.String() == "" {
		return errors.New("missing receiverRequest.DefaultDepository")
	}
	return nil
}

func AddReceiverRoutes(logger log.Logger, r *mux.Router, customersClient customers.Client, depositoryRepo DepositoryRepository, receiverRepo receiverRepository) {
	r.Methods("GET").Path("/receivers").HandlerFunc(getUserReceivers(logger, receiverRepo))
	r.Methods("POST").Path("/receivers").HandlerFunc(createUserReceiver(logger, customersClient, depositoryRepo, receiverRepo))

	r.Methods("GET").Path("/receivers/{receiverId}").HandlerFunc(getUserReceiver(logger, receiverRepo))
	r.Methods("PATCH").Path("/receivers/{receiverId}").HandlerFunc(updateUserReceiver(logger, depositoryRepo, receiverRepo))
	r.Methods("DELETE").Path("/receivers/{receiverId}").HandlerFunc(deleteUserReceiver(logger, receiverRepo))
}

func getUserReceivers(logger log.Logger, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		receivers, err := receiverRepo.getUserReceivers(responder.XUserID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(receivers)
		})
	}
}

func readReceiverRequest(r *http.Request) (receiverRequest, error) {
	var wrapper receiverRequest
	if err := json.NewDecoder(Read(r.Body)).Decode(&wrapper); err != nil {
		return wrapper, err
	}
	if err := wrapper.missingFields(); err != nil {
		return wrapper, fmt.Errorf("%v: %v", ErrMissingRequiredJson, err)
	}
	return wrapper, nil
}

// parseAndValidateEmail attempts to parse an email address and validate the domain name.
func parseAndValidateEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return "", fmt.Errorf("error parsing '%s': %v", raw, err)
	}
	return addr.Address, nil
}

func createUserReceiver(logger log.Logger, customersClient customers.Client, depositoryRepo DepositoryRepository, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		req, err := readReceiverRequest(r)
		if err != nil {
			responder.Log("receivers", fmt.Errorf("error reading receiverRequest: %v", err))
			responder.Problem(err)
			return
		}

		dep, err := depositoryRepo.GetUserDepository(req.DefaultDepository, responder.XUserID)
		if err != nil || dep == nil {
			responder.Log("receivers", "depository not found")
			responder.Problem(errors.New("depository not found"))
			return
		}

		email, err := parseAndValidateEmail(req.Email)
		if err != nil {
			responder.Log("receivers", fmt.Sprintf("unable to validate receiver email: %v", err))
			responder.Problem(err)
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
			responder.Log("receivers", fmt.Errorf("error validating Receiver: %v", err))
			responder.Problem(err)
			return
		}

		// Add the Receiver into our Customers service
		if customersClient != nil {
			customer, err := customersClient.Create(&customers.Request{
				Name:      dep.Holder,
				BirthDate: req.BirthDate,
				Addresses: model.ConvertAddress(req.Address),
				Email:     email,
				RequestID: responder.XRequestID,
				UserID:    responder.XUserID,
			})
			if err != nil || customer == nil {
				responder.Log("receivers", "error creating customer", "error", err)
				responder.Problem(err)
				return
			}
			responder.Log("receivers", fmt.Sprintf("created customer=%s", customer.ID))
			receiver.CustomerID = customer.ID
		} else {
			responder.Log("receivers", "skipped adding receiver into Customers")
		}

		if err := receiverRepo.upsertUserReceiver(responder.XUserID, receiver); err != nil {
			err = fmt.Errorf("creating receiver=%s, user_id=%s: %v", receiver.ID, responder.XUserID, err)
			responder.Log("receivers", fmt.Errorf("error inserting Receiver: %v", err))
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(receiver)
		})
	}
}

func getUserReceiver(logger log.Logger, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		receiverID := getReceiverID(r)
		if receiverID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver, err := receiverRepo.getUserReceiver(receiverID, responder.XUserID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(receiver)
		})
	}
}

func updateUserReceiver(logger log.Logger, depRepo DepositoryRepository, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		var wrapper receiverRequest
		if err := json.NewDecoder(Read(r.Body)).Decode(&wrapper); err != nil {
			responder.Problem(err)
			return
		}

		receiverID := getReceiverID(r)
		if receiverID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver, err := receiverRepo.getUserReceiver(receiverID, responder.XUserID)
		if receiver == nil || err != nil {
			responder.Log("receivers", fmt.Sprintf("problem getting receiver='%s': %v", receiverID, err))
			responder.Problem(err)
			return
		}
		if wrapper.DefaultDepository != "" {
			// Verify the user controls the requested Depository
			dep, err := depRepo.GetUserDepository(wrapper.DefaultDepository, responder.XUserID)
			if err != nil || dep == nil {
				responder.Log("receivers", "depository doesn't belong to user")
				responder.Problem(errors.New("depository not found"))
				return
			}
			receiver.DefaultDepository = wrapper.DefaultDepository
		}
		if wrapper.Metadata != "" {
			receiver.Metadata = wrapper.Metadata
		}
		receiver.Updated = base.NewTime(time.Now())

		if err := receiver.validate(); err != nil {
			responder.Log("receivers", fmt.Sprintf("problem validating updatable receiver=%s: %v", receiver.ID, err))
			responder.Problem(err)
			return
		}

		// Perform update
		if err := receiverRepo.upsertUserReceiver(responder.XUserID, receiver); err != nil {
			responder.Log("receivers", fmt.Sprintf("problem upserting receiver=%s: %v", receiver.ID, err))
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(receiver)
		})
	}
}

func deleteUserReceiver(logger log.Logger, receiverRepo receiverRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		if receiverID := getReceiverID(r); receiverID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			if err := receiverRepo.deleteUserReceiver(receiverID, responder.XUserID); err != nil {
				responder.Problem(err)
				return
			}
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
		})
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
	getUserReceivers(userID id.User) ([]*Receiver, error)
	getUserReceiver(id ReceiverID, userID id.User) (*Receiver, error)

	updateReceiverStatus(id ReceiverID, status ReceiverStatus) error

	upsertUserReceiver(userID id.User, receiver *Receiver) error
	deleteUserReceiver(id ReceiverID, userID id.User) error
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

func (r *SQLReceiverRepo) getUserReceivers(userID id.User) ([]*Receiver, error) {
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

func (r *SQLReceiverRepo) getUserReceiver(id ReceiverID, userID id.User) (*Receiver, error) {
	query := `select receiver_id, email, default_depository, customer_id, status, metadata, created_at, last_updated_at
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
	err = row.Scan(&receiver.ID, &receiver.Email, &receiver.DefaultDepository, &receiver.CustomerID, &receiver.Status, &receiver.Metadata, &receiver.Created.Time, &receiver.Updated.Time)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if receiver.ID == "" || receiver.Email == "" {
		return nil, nil // no records found
	}
	return &receiver, nil
}

func (r *SQLReceiverRepo) updateReceiverStatus(id ReceiverID, status ReceiverStatus) error {
	query := `update receivers set status = ?, last_updated_at = ? where receiver_id = ? and deleted_at is null;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(status, time.Now(), id); err != nil {
		return fmt.Errorf("error updating receiver=%s: %v", id, err)
	}
	return nil
}

func (r *SQLReceiverRepo) upsertUserReceiver(userID id.User, receiver *Receiver) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	receiver.Updated = base.NewTime(time.Now().Truncate(1 * time.Second))

	query := `insert into receivers (receiver_id, user_id, email, default_depository, customer_id, status, metadata, created_at, last_updated_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("upsertUserReceiver: prepare err=%v: rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()

	res, err := stmt.Exec(receiver.ID, userID, receiver.Email, receiver.DefaultDepository, receiver.CustomerID, receiver.Status, receiver.Metadata, receiver.Created.Time, receiver.Updated.Time)
	stmt.Close()
	if err != nil && !database.UniqueViolation(err) {
		return fmt.Errorf("problem upserting receiver=%q, userID=%q error=%v rollback=%v", receiver.ID, userID, err, tx.Rollback())
	}

	// Check and skip ahead if the insert failed (to database.UniqueViolation)
	if res != nil {
		if n, _ := res.RowsAffected(); n != 0 {
			return tx.Commit() // Receiver was inserted, so cleanup and exit
		}
	}
	query = `update receivers
set email = ?, default_depository = ?, customer_id = ?, status = ?, metadata = ?, last_updated_at = ?
where receiver_id = ? and user_id = ? and deleted_at is null`
	stmt, err = tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(receiver.Email, receiver.DefaultDepository, receiver.CustomerID, receiver.Status, receiver.Metadata, receiver.Updated.Time, receiver.ID, userID)
	stmt.Close()
	if err != nil {
		return fmt.Errorf("upsertUserReceiver: exec error=%v rollback=%v", err, tx.Rollback())
	}
	return tx.Commit()
}

func (r *SQLReceiverRepo) deleteUserReceiver(id ReceiverID, userID id.User) error {
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

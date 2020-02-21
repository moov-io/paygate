// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package receivers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"time"

	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

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

func AddReceiverRoutes(logger log.Logger, r *mux.Router, customersClient customers.Client, depositoryRepo depository.Repository, receiverRepo Repository) {
	r.Methods("GET").Path("/receivers").HandlerFunc(getUserReceivers(logger, receiverRepo))
	r.Methods("POST").Path("/receivers").HandlerFunc(createUserReceiver(logger, customersClient, depositoryRepo, receiverRepo))

	r.Methods("GET").Path("/receivers/{receiverId}").HandlerFunc(GetUserReceiver(logger, receiverRepo))
	r.Methods("PATCH").Path("/receivers/{receiverId}").HandlerFunc(updateUserReceiver(logger, depositoryRepo, receiverRepo))
	r.Methods("DELETE").Path("/receivers/{receiverId}").HandlerFunc(deleteUserReceiver(logger, receiverRepo))
}

func getUserReceivers(logger log.Logger, receiverRepo Repository) http.HandlerFunc {
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
	if err := json.NewDecoder(route.Read(r.Body)).Decode(&wrapper); err != nil {
		return wrapper, err
	}
	if err := wrapper.missingFields(); err != nil {
		return wrapper, fmt.Errorf("%v: %v", route.ErrMissingRequiredJson, err)
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

func createUserReceiver(logger log.Logger, customersClient customers.Client, depositoryRepo depository.Repository, receiverRepo Repository) http.HandlerFunc {
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
		receiver := &model.Receiver{
			ID:                model.ReceiverID(base.ID()),
			Email:             email,
			DefaultDepository: req.DefaultDepository,
			Status:            model.ReceiverUnverified,
			Metadata:          req.Metadata,
			Created:           base.NewTime(time.Now()),
		}
		if err := receiver.Validate(); err != nil {
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

		if err := receiverRepo.UpsertUserReceiver(responder.XUserID, receiver); err != nil {
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

func GetUserReceiver(logger log.Logger, receiverRepo Repository) http.HandlerFunc {
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

		receiver, err := receiverRepo.GetUserReceiver(receiverID, responder.XUserID)
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

func updateUserReceiver(logger log.Logger, depRepo depository.Repository, receiverRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		var wrapper receiverRequest
		if err := json.NewDecoder(route.Read(r.Body)).Decode(&wrapper); err != nil {
			responder.Problem(err)
			return
		}

		receiverID := getReceiverID(r)
		if receiverID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver, err := receiverRepo.GetUserReceiver(receiverID, responder.XUserID)
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

		if err := receiver.Validate(); err != nil {
			responder.Log("receivers", fmt.Sprintf("problem validating updatable receiver=%s: %v", receiver.ID, err))
			responder.Problem(err)
			return
		}

		// Perform update
		if err := receiverRepo.UpsertUserReceiver(responder.XUserID, receiver); err != nil {
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

func deleteUserReceiver(logger log.Logger, receiverRepo Repository) http.HandlerFunc {
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
func getReceiverID(r *http.Request) model.ReceiverID {
	v := mux.Vars(r)
	id, ok := v["receiverId"]
	if !ok {
		return model.ReceiverID("")
	}
	return model.ReceiverID(id)
}

type Repository interface {
	getUserReceivers(userID id.User) ([]*model.Receiver, error)
	GetUserReceiver(id model.ReceiverID, userID id.User) (*model.Receiver, error)

	UpdateReceiverStatus(id model.ReceiverID, status model.ReceiverStatus) error

	UpsertUserReceiver(userID id.User, receiver *model.Receiver) error
	deleteUserReceiver(id model.ReceiverID, userID id.User) error
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

func (r *SQLReceiverRepo) getUserReceivers(userID id.User) ([]*model.Receiver, error) {
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

	var receivers []*model.Receiver
	for i := range receiverIDs {
		receiver, err := r.GetUserReceiver(model.ReceiverID(receiverIDs[i]), userID)
		if err == nil && receiver != nil && receiver.Email != "" {
			receivers = append(receivers, receiver)
		}
	}
	return receivers, rows.Err()
}

func (r *SQLReceiverRepo) GetUserReceiver(id model.ReceiverID, userID id.User) (*model.Receiver, error) {
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

	var receiver model.Receiver
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

func (r *SQLReceiverRepo) UpdateReceiverStatus(id model.ReceiverID, status model.ReceiverStatus) error {
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

func (r *SQLReceiverRepo) UpsertUserReceiver(userID id.User, receiver *model.Receiver) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	receiver.Updated = base.NewTime(time.Now().Truncate(1 * time.Second))

	query := `insert into receivers (receiver_id, user_id, email, default_depository, customer_id, status, metadata, created_at, last_updated_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("UpsertUserReceiver: prepare err=%v: rollback=%v", err, tx.Rollback())
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
		return fmt.Errorf("UpsertUserReceiver: exec error=%v rollback=%v", err, tx.Rollback())
	}
	return tx.Commit()
}

func (r *SQLReceiverRepo) deleteUserReceiver(id model.ReceiverID, userID id.User) error {
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

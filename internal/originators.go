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
	"time"

	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/ofac"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type OriginatorID string

// Originator objects are an organization or person that initiates
// an ACH Transfer to a Receiver account either as a debit or credit.
// The API allows you to create, delete, and update your originators.
// You can retrieve individual originators as well as a list of all your
// originators. (Batch Header)
type Originator struct {
	// ID is a unique string representing this Originator.
	ID OriginatorID `json:"id"`

	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository DepositoryID `json:"defaultDepository"`

	// Identification is a number by which the receiver is known to the originator
	// This should be the 9 digit FEIN number for a company or Social Security Number for an Individual
	Identification string `json:"identification"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated base.Time `json:"updated"`
}

func (o *Originator) missingFields() error {
	if o.DefaultDepository == "" {
		return errors.New("missing Originator.DefaultDepository")
	}
	if o.Identification == "" {
		return errors.New("missing Originator.Identification")
	}
	return nil
}

func (o *Originator) validate() error {
	if o == nil {
		return errors.New("nil Originator")
	}
	if err := o.missingFields(); err != nil {
		return err
	}
	if o.Identification == "" {
		return errors.New("misisng Originator.Identification")
	}
	return nil
}

type originatorRequest struct {
	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository DepositoryID `json:"defaultDepository"`

	// Identification is a number by which the receiver is known to the originator
	Identification string `json:"identification"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`
}

func (r originatorRequest) missingFields() error {
	if r.Identification == "" {
		return errors.New("missing originatorRequest.Identification")
	}
	if r.DefaultDepository.empty() {
		return errors.New("missing originatorRequest.DefaultDepository")
	}
	return nil
}

func AddOriginatorRoutes(logger log.Logger, r *mux.Router, accountsClient AccountsClient, ofacClient ofac.Client, depositoryRepo DepositoryRepository, originatorRepo originatorRepository) {
	r.Methods("GET").Path("/originators").HandlerFunc(getUserOriginators(logger, originatorRepo))
	r.Methods("POST").Path("/originators").HandlerFunc(createUserOriginator(logger, accountsClient, ofacClient, originatorRepo, depositoryRepo))

	r.Methods("GET").Path("/originators/{originatorId}").HandlerFunc(getUserOriginator(logger, originatorRepo))
	r.Methods("DELETE").Path("/originators/{originatorId}").HandlerFunc(deleteUserOriginator(logger, originatorRepo))
}

func getUserOriginators(logger log.Logger, originatorRepo originatorRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		requestID, userID := moovhttp.GetRequestID(r), moovhttp.GetUserID(r)
		origs, err := originatorRepo.getUserOriginators(userID)
		if err != nil {
			logger.Log("originators", fmt.Sprintf("problem reading user originators: %v", err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(origs)
	}
}

func readOriginatorRequest(r *http.Request) (originatorRequest, error) {
	var req originatorRequest
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

func createUserOriginator(logger log.Logger, accountsClient AccountsClient, ofacClient ofac.Client, originatorRepo originatorRepository, depositoryRepo DepositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		req, err := readOriginatorRequest(r)
		if err != nil {
			logger.Log("originators", err.Error())
			moovhttp.Problem(w, err)
			return
		}

		userID, requestID := moovhttp.GetUserID(r), moovhttp.GetRequestID(r)

		// Verify depository belongs to the user
		dep, err := depositoryRepo.GetUserDepository(req.DefaultDepository, userID)
		if err != nil || dep == nil || dep.ID != req.DefaultDepository {
			moovhttp.Problem(w, fmt.Errorf("depository %s does not exist", req.DefaultDepository))
			return
		}

		// Verify account exists in Accounts for receiver (userID)
		if accountsClient != nil {
			account, err := accountsClient.SearchAccounts(requestID, userID, dep)
			if err != nil || account == nil {
				logger.Log("originators", fmt.Sprintf("problem finding account depository=%s: %v", dep.ID, err), "requestID", requestID, "userID", userID)
				moovhttp.Problem(w, err)
				return
			}
		}

		// Check OFAC for customer/company data
		if err := ofac.RejectViaMatch(logger, ofacClient, req.Metadata, userID, requestID); err != nil {
			logger.Log("originators", fmt.Sprintf("error checking OFAC for '%s': %v", req.Metadata, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		// Write Originator to DB
		orig, err := originatorRepo.createUserOriginator(userID, req)
		if err != nil {
			logger.Log("originators", fmt.Sprintf("problem creating originator: %v", err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(orig)
	}
}

func getUserOriginator(logger log.Logger, originatorRepo originatorRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		id, userID := getOriginatorId(r), moovhttp.GetUserID(r)
		requestID := moovhttp.GetRequestID(r)
		orig, err := originatorRepo.getUserOriginator(id, userID)
		if err != nil {
			logger.Log("originators", fmt.Sprintf("problem reading originator=%s: %v", id, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(orig)
	}
}

func deleteUserOriginator(logger log.Logger, originatorRepo originatorRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		id, userID := getOriginatorId(r), moovhttp.GetUserID(r)
		requestID := moovhttp.GetRequestID(r)
		if err := originatorRepo.deleteUserOriginator(id, userID); err != nil {
			logger.Log("originators", fmt.Sprintf("problem deleting originator=%s: %v", id, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func getOriginatorId(r *http.Request) OriginatorID {
	vars := mux.Vars(r)
	v, ok := vars["originatorId"]
	if ok {
		return OriginatorID(v)
	}
	return OriginatorID("")
}

type originatorRepository interface {
	getUserOriginators(userID string) ([]*Originator, error)
	getUserOriginator(id OriginatorID, userID string) (*Originator, error)

	createUserOriginator(userID string, req originatorRequest) (*Originator, error)
	deleteUserOriginator(id OriginatorID, userID string) error
}

func NewOriginatorRepo(logger log.Logger, db *sql.DB) *SQLOriginatorRepo {
	return &SQLOriginatorRepo{log: logger, db: db}
}

type SQLOriginatorRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *SQLOriginatorRepo) Close() error {
	return r.db.Close()
}

func (r *SQLOriginatorRepo) getUserOriginators(userID string) ([]*Originator, error) {
	query := `select originator_id from originators where user_id = ? and deleted_at is null`
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

	var originatorIds []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return nil, fmt.Errorf("getUserOriginators scan: %v", err)
		}
		if row != "" {
			originatorIds = append(originatorIds, row)
		}
	}

	var originators []*Originator
	for i := range originatorIds {
		orig, err := r.getUserOriginator(OriginatorID(originatorIds[i]), userID)
		if err == nil && orig.ID != "" {
			originators = append(originators, orig)
		}
	}
	return originators, rows.Err()
}

func (r *SQLOriginatorRepo) getUserOriginator(id OriginatorID, userID string) (*Originator, error) {
	query := `select originator_id, default_depository, identification, metadata, created_at, last_updated_at
from originators
where originator_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userID)

	orig := &Originator{}
	var (
		created time.Time
		updated time.Time
	)
	err = row.Scan(&orig.ID, &orig.DefaultDepository, &orig.Identification, &orig.Metadata, &created, &updated)
	if err != nil {
		return nil, err
	}
	orig.Created = base.NewTime(created)
	orig.Updated = base.NewTime(updated)
	if orig.ID == "" {
		return nil, nil // not found
	}
	return orig, nil
}

func (r *SQLOriginatorRepo) createUserOriginator(userID string, req originatorRequest) (*Originator, error) {
	now := time.Now()
	orig := &Originator{
		ID:                OriginatorID(base.ID()),
		DefaultDepository: req.DefaultDepository,
		Identification:    req.Identification,
		Metadata:          req.Metadata,
		Created:           base.NewTime(now),
		Updated:           base.NewTime(now),
	}
	if err := orig.validate(); err != nil {
		return nil, err
	}

	query := `insert into originators (originator_id, user_id, default_depository, identification, metadata, created_at, last_updated_at) values (?, ?, ?, ?, ?, ?, ?)`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	_, err = stmt.Exec(orig.ID, userID, orig.DefaultDepository, orig.Identification, orig.Metadata, now, now)
	if err != nil {
		return nil, err
	}
	return orig, nil
}

func (r *SQLOriginatorRepo) deleteUserOriginator(id OriginatorID, userID string) error {
	query := `update originators set deleted_at = ? where originator_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(time.Now(), id, userID)
	return err
}

// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type OriginatorID string

// Originator objects are an organization or person that initiates
// an ACH Transfer to a Customer account either as a debit or credit.
// The API allows you to create, delete, and update your originators.
// You can retrieve individual originators as well as a list of all your
// originators. (Batch Header)
type Originator struct {
	// ID is a unique string representing this Originator.
	ID OriginatorID `json:"id"`

	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository DepositoryID `json:"defaultDepository"` // TODO(adam): validate

	// Identification is a number by which the customer is known to the originator
	Identification string `json:"identification"` // TODO(adam): validate? not-blank?

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created time.Time `json:"created"`

	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated time.Time `json:"updated"`
}

type originatorRequest struct {
	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository DepositoryID `json:"defaultDepository"`

	// Identification is a number by which the customer is known to the originator
	Identification string `json:"identification"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`
}

func (r originatorRequest) missingFields() bool {
	return r.DefaultDepository.empty() || r.Identification == ""
}

func addOriginatorRoutes(r *mux.Router, originatorRepo originatorRepository) {
	r.Methods("GET").Path("/originators").HandlerFunc(getUserOriginators(originatorRepo))
	r.Methods("POST").Path("/originators").HandlerFunc(createUserOriginator(originatorRepo))

	r.Methods("GET").Path("/originators/{originatorId}").HandlerFunc(getUserOriginator(originatorRepo))
	r.Methods("DELETE").Path("/originators/{originatorId}").HandlerFunc(deleteUserOriginator(originatorRepo))
}

func getUserOriginators(originatorRepo originatorRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserOriginators")
		if err != nil {
			return
		}

		userId := getUserId(r)
		origs, err := originatorRepo.getUserOriginators(userId)
		if err != nil {
			internalError(w, err, "getUserOriginators")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(origs); err != nil {
			internalError(w, err, "getUserOriginators")
			return
		}
	}
}

func createUserOriginator(originatorRepo originatorRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserOriginator")
		if err != nil {
			return
		}

		bs, err := read(r.Body)
		if err != nil {
			encodeError(w, err)
			return
		}
		var req originatorRequest
		if err := json.Unmarshal(bs, &req); err != nil {
			encodeError(w, err)
			return
		}

		if req.missingFields() {
			encodeError(w, errMissingRequiredJson)
			return
		}

		userId := getUserId(r)
		orig, err := originatorRepo.createUserOriginator(userId, req)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(orig); err != nil {
			internalError(w, err, "createUserOriginator")
			return
		}
	}
}

func getUserOriginator(originatorRepo originatorRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserOriginator")
		if err != nil {
			return
		}

		id, userId := getOriginatorId(r), getUserId(r)
		orig, err := originatorRepo.getUserOriginator(id, userId)
		if err != nil {
			internalError(w, err, "getUserOriginator")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(orig); err != nil {
			internalError(w, err, "getUserOriginator")
			return
		}
	}
}

func deleteUserOriginator(originatorRepo originatorRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserOriginator")
		if err != nil {
			return
		}

		id, userId := getOriginatorId(r), getUserId(r)
		if err := originatorRepo.deleteUserOriginator(id, userId); err != nil {
			internalError(w, err, "deleteUserOriginator")
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
	getUserOriginators(userId string) ([]*Originator, error)
	getUserOriginator(id OriginatorID, userId string) (*Originator, error)

	createUserOriginator(userId string, req originatorRequest) (*Originator, error)
	deleteUserOriginator(id OriginatorID, userId string) error
}

type sqliteOriginatorRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *sqliteOriginatorRepo) close() error {
	return r.db.Close()
}

func (r *sqliteOriginatorRepo) getUserOriginators(userId string) ([]*Originator, error) {
	query := `select originator_id from originators where user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	rows, err := stmt.Query(userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var originatorIds []string
	for rows.Next() {
		var row string
		rows.Scan(&row)
		if row != "" {
			originatorIds = append(originatorIds, row)
		}
	}

	var originators []*Originator
	for i := range originatorIds {
		orig, err := r.getUserOriginator(OriginatorID(originatorIds[i]), userId)
		if err == nil && orig.ID != "" {
			originators = append(originators, orig)
		}
	}
	return originators, nil
}

func (r *sqliteOriginatorRepo) getUserOriginator(id OriginatorID, userId string) (*Originator, error) {
	query := `select originator_id, default_depository, identification, metadata, created_at, last_updated_at
from originators
where originator_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	row := stmt.QueryRow(id, userId)

	orig := &Originator{}
	err = row.Scan(&orig.ID, &orig.DefaultDepository, &orig.Identification, &orig.Metadata, &orig.Created, &orig.Updated)
	if err != nil {
		return nil, err
	}
	if orig.ID == "" {
		return nil, nil // not found
	}
	return orig, nil
}

func (r *sqliteOriginatorRepo) createUserOriginator(userId string, req originatorRequest) (*Originator, error) {
	originatorId, now := nextID(), time.Now()
	query := `insert into originators (originator_id, user_id, default_depository, identification, metadata, created_at, last_updated_at) values (?, ?, ?, ?, ?, ?, ?)`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	_, err = stmt.Exec(originatorId, userId, req.DefaultDepository, req.Identification, req.Metadata, now, now)
	if err != nil {
		return nil, err
	}
	return &Originator{
		ID:                OriginatorID(originatorId),
		DefaultDepository: req.DefaultDepository,
		Identification:    req.Identification,
		Metadata:          req.Metadata,
		Created:           now,
		Updated:           now,
	}, nil
}

func (r *sqliteOriginatorRepo) deleteUserOriginator(id OriginatorID, userId string) error {
	query := `update originators set deleted_at = ? where originator_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(time.Now(), id, userId)
	return err
}

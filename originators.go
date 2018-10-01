// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type OriginatorID string

// Originator objects are an organization or person that initiates
// an ACH Transfer to a Customer account either as a debit or credit.
// The API allows you to create, delete, and update your originators.
// You can retrieve individual originators as well as a list of all your
// originators. (Batch Header)
type Originator struct {
	// ID is a globally unique identifier
	ID OriginatorID `json:"id"`

	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository DepositoryID `json:"defaultDepository"`

	// Identification is a number by which the customer is known to the originator
	Identification string `json:"identification"`

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

type memOriginatorRepo struct{}

func (r memOriginatorRepo) getUserOriginators(userId string) ([]*Originator, error) {
	orig, err := r.getUserOriginator(OriginatorID(nextID()), userId)
	if err != nil {
		return nil, err
	}
	return []*Originator{orig}, nil
}

func (r memOriginatorRepo) getUserOriginator(id OriginatorID, userId string) (*Originator, error) {
	return &Originator{
		ID:                id,
		DefaultDepository: DepositoryID(nextID()),
		Identification:    "identify",
		Created:           time.Now(),
	}, nil
}

func (r memOriginatorRepo) createUserOriginator(userId string, req originatorRequest) (*Originator, error) {
	return r.getUserOriginator(OriginatorID(nextID()), userId)
}

func (r memOriginatorRepo) deleteUserOriginator(id OriginatorID, userId string) error {
	return nil
}

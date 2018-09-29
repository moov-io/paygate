// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type DepositoryID string

func (id DepositoryID) empty() bool {
	return string(id) == ""
}

type Depository struct {
	ID            DepositoryID     `json:"id"`
	BankName      string           `json:"bankName"`
	Holder        string           `json:"holder"`
	HolderType    HolderType       `json:"holderType"`
	Type          AccountType      `json:"type"`
	RoutingNumber string           `json:"routingNumber"`
	AccountNumber string           `json:"accountNumber"`
	Status        DepositoryStatus `json:"status"`
	Metadata      string           `json:"metadata"`
	Parent        *DepositoryID    `json:"parent"`
	Created       time.Time        `json:"created"`
	Updated       time.Time        `json:"updated"`
}

type depositoryRequest struct { // TODO(adam): we need to update the openapi docs
	BankName      string        `json:"bankName,omitempty"`
	Holder        string        `json:"holder,omitempty"`
	HolderType    HolderType    `json:"holderType,omitempty"`
	Type          AccountType   `json:"type,omitempty"`
	RoutingNumber string        `json:"routingNumber,omitempty"`
	AccountNumber string        `json:"accountNumber,omitempty"`
	Metadata      string        `json:"metadata,omitempty"`
	Parent        *DepositoryID `json:"parent,omitempty"`
}

func (r depositoryRequest) missingFields() bool {
	empty := func(s string) bool { return s == "" }
	return (empty(r.BankName) ||
		empty(r.Holder) ||
		r.HolderType.empty() ||
		r.Type.empty() ||
		empty(r.RoutingNumber) ||
		empty(r.AccountNumber))
}

type HolderType string

const (
	Individual HolderType = "Individual"
	Business              = "Business"
)

func (t *HolderType) empty() bool {
	return string(*t) == ""
}

func (t *HolderType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "individual":
		*t = Individual
		return nil
	case "business":
		*t = Business
		return nil
	}
	return fmt.Errorf("unknown HolderType %q", s)
}

type DepositoryStatus string

const (
	DepositoryUnverified DepositoryStatus = "Unverified"
	DepositoryVerified                    = "Verified"
)

func (ds DepositoryStatus) empty() bool {
	return string(ds) == ""
}

func (ds *DepositoryStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "unverified":
		*ds = DepositoryUnverified
		return nil
	case "verified":
		*ds = DepositoryVerified
		return nil
	}
	return fmt.Errorf("unknown DepositoryStatus %q", s)
}

func addDepositoryRoutes(r *mux.Router, depositRepo depositoryRepository) {
	r.Methods("GET").Path("/depositories").HandlerFunc(getUserDepositories(depositRepo))
	r.Methods("POST").Path("/depositories").HandlerFunc(createUserDepository(depositRepo))

	r.Methods("GET").Path("/depositories/{depositoryId}").HandlerFunc(getUserDepository(depositRepo))
	r.Methods("PATCH").Path("/depositories/{depositoryId}").HandlerFunc(updateUserDepository(depositRepo))
	r.Methods("DELETE").Path("/depositories/{depositoryId}").HandlerFunc(deleteUserDepository(depositRepo))

	r.Methods("POST").Path("/depositories/{depositoryId}/micro-deposits").HandlerFunc(initiateMicroDeposits(depositRepo))
}

// GET /depositories
// response: [ depository ]
func getUserDepositories(depositRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserDepositories")
		if err != nil {
			return
		}

		userId := getUserId(r)
		deposits, err := depositRepo.getUserDeposits(userId)
		if err != nil {
			internalError(w, err, "getUserDepositories")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(deposits); err != nil {
			internalError(w, err, "getUserDepositories")
			return
		}
	}
}

// POST /depositories
// request: model w/o ID
// response: 201 w/ depository json
func createUserDepository(depositRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserDepository")
		if err != nil {
			return
		}

		bs, err := read(r.Body)
		if err != nil {
			encodeError(w, err)
			return
		}

		var req depositoryRequest
		if err := json.Unmarshal(bs, &req); err != nil {
			encodeError(w, err)
			return
		}
		if req.missingFields() {
			encodeError(w, errMissingRequiredJson)
			return
		}

		userId := getUserId(r)
		deposit, err := depositRepo.createUserDeposit(userId, req)
		if err != nil {
			internalError(w, err, "createUserDepository")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(deposit); err != nil {
			internalError(w, err, "createUserDepository")
			return
		}
	}
}

func getUserDepository(depositRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserDepository")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		deposit, err := depositRepo.getUserDeposit(id, userId)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(deposit); err != nil {
			internalError(w, err, "getUserDepository")
			return
		}
	}
}

func updateUserDepository(depositRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "updateUserDepository")
		if err != nil {
			return
		}

		bs, err := read(r.Body)
		if err != nil {
			encodeError(w, err)
			return
		}
		var req depositoryRequest
		if err := json.Unmarshal(bs, &req); err != nil {
			encodeError(w, err)
			return
		}

		id, userId := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		deposit, err := depositRepo.upsertUserDeposit(id, userId, req)
		if err != nil {
			internalError(w, err, "updateUserDepository")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(deposit); err != nil {
			internalError(w, err, "updateUserDepository")
			return
		}
	}
}

func deleteUserDepository(depositRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserDepository")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := depositRepo.deleteUserDeposit(id, userId); err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
	}
}

// POST /depositories/{id}/micro-deposits
// 200 - Micro deposits verified
// 201 - Micro deposits initiated
// 400 - Invalid Amounts
// 404 - A depository with the specified ID was not found.
// 409 - Too many attempts. Bank already verified.
func initiateMicroDeposits(depositRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserDepository")
		if err != nil {
			return
		}

		id, _ := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// TODO(adam): something
		// if err := depositRepo.initiateMicroDeposits(id, userId); err != nil {
		// 	// TODO(adam)
		// }

		switch id {
		case "200":
			w.WriteHeader(http.StatusOK)
		case "201":
			w.WriteHeader(http.StatusCreated)
		case "400":
			w.WriteHeader(http.StatusBadRequest)
		case "404":
			w.WriteHeader(http.StatusNotFound)
		case "409":
			w.WriteHeader(http.StatusConflict)
		}
	}
}

// getDepositoryId extracts the DepositoryID from the incoming request.
func getDepositoryId(r *http.Request) DepositoryID {
	v := mux.Vars(r)
	id, ok := v["depositoryId"]
	if !ok {
		return DepositoryID("")
	}
	return DepositoryID(id)
}

type depositoryRepository interface {
	getUserDeposits(userId string) ([]*Depository, error)
	getUserDeposit(id DepositoryID, userId string) (*Depository, error)

	createUserDeposit(userId string, req depositoryRequest) (*Depository, error)
	upsertUserDeposit(id DepositoryID, userId string, req depositoryRequest) (*Depository, error)
	deleteUserDeposit(id DepositoryID, userId string) error

	initiateMicroDeposits(id DepositoryID, userId string) error
}

type memDepositoryRepo struct{}

func (m memDepositoryRepo) getUserDeposits(userId string) ([]*Depository, error) {
	d, err := m.getUserDeposit(DepositoryID(nextID()), userId)
	if err != nil {
		return nil, err
	}
	return []*Depository{d}, nil
}

func (memDepositoryRepo) getUserDeposit(id DepositoryID, userId string) (*Depository, error) {
	return &Depository{
		ID:            id,
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       time.Now().Add(-1 * time.Second),
	}, nil
}

func (m memDepositoryRepo) createUserDeposit(userId string, req depositoryRequest) (*Depository, error) {
	return m.getUserDeposit(DepositoryID(nextID()), userId)
}

func (m memDepositoryRepo) upsertUserDeposit(id DepositoryID, userId string, req depositoryRequest) (*Depository, error) {
	return m.getUserDeposit(id, userId)
}

func (memDepositoryRepo) deleteUserDeposit(id DepositoryID, userId string) error {
	return nil
}

func (memDepositoryRepo) initiateMicroDeposits(id DepositoryID, userId string) error {
	return nil
}

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

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type DepositoryID string

func (id DepositoryID) empty() bool {
	return string(id) == ""
}

type Depository struct {
	// ID is a unique string representing this Depository.
	ID DepositoryID `json:"id"`

	// BankName is the legal name of the financial institution.
	BankName string `json:"bankName"`

	// Holder is the legal holder name on the account
	Holder string `json:"holder"`

	// HolderType defines the type of entity of the account holder as an individual or company
	HolderType HolderType `json:"holderType"`

	// Type defines the account as checking or savings
	Type AccountType `json:"type"`

	// RoutingNumber is the ABA routing transit number for the depository account.
	RoutingNumber string `json:"routingNumber"`

	// AccountNumber is the account number for the depository account
	AccountNumber string `json:"accountNumber"`

	// Status defines the current state of the Depository
	Status DepositoryStatus `json:"status"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`

	// Parent is the depository owner's valid Customer ID or Originator ID. Used for search and display purposes.
	Parent *DepositoryID `json:"parent"` // TODO(adam): change type(s) ?

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated base.Time `json:"updated"`
}

func (d *Depository) validate() error {
	if err := d.HolderType.validate(); err != nil {
		return err
	}
	if err := d.Type.validate(); err != nil {
		return err
	}
	if err := d.Status.validate(); err != nil {
		return err
	}
	if err := ach.CheckRoutingNumber(d.RoutingNumber); err != nil {
		return err
	}
	if d.AccountNumber == "" {
		return errors.New("missing Depository.AccountNumber")
	}
	// TODO(adam): check d.Parent.validate() (if d.Parent != nil)
	return nil
}

type depositoryRequest struct {
	BankName      string        `json:"bankName,omitempty"`
	Holder        string        `json:"holder,omitempty"`
	HolderType    HolderType    `json:"holderType,omitempty"`
	Type          AccountType   `json:"type,omitempty"`
	RoutingNumber string        `json:"routingNumber,omitempty"`
	AccountNumber string        `json:"accountNumber,omitempty"`
	Metadata      string        `json:"metadata,omitempty"`
	Parent        *DepositoryID `json:"parent,omitempty"`
}

func (r depositoryRequest) missingFields() error {
	if r.BankName == "" {
		return errors.New("missing depositoryRequest.BankName")
	}
	if r.Holder == "" {
		return errors.New("missing depositoryRequest.Holder")
	}
	if r.HolderType == "" {
		return errors.New("missing depositoryRequest.HolderType")
	}
	if r.Type == "" {
		return errors.New("missing depositoryRequest.Type")
	}
	if r.RoutingNumber == "" {
		return errors.New("missing depositoryRequest.RoutingNumber")
	}
	if r.AccountNumber == "" {
		return errors.New("missing depositoryRequest.AccountNumber")
	}
	return nil
}

type HolderType string

const (
	Individual HolderType = "individual"
	Business   HolderType = "business"
)

func (t *HolderType) empty() bool {
	return string(*t) == ""
}

func (t HolderType) validate() error {
	switch t {
	case Individual, Business:
		return nil
	default:
		return fmt.Errorf("HolderType(%s) is invalid", t)
	}
}

func (t *HolderType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = HolderType(strings.ToLower(s))
	if err := t.validate(); err != nil {
		return err
	}
	return nil
}

type DepositoryStatus string

const (
	DepositoryUnverified DepositoryStatus = "unverified"
	DepositoryVerified   DepositoryStatus = "verified"
)

func (ds DepositoryStatus) empty() bool {
	return string(ds) == ""
}

func (ds DepositoryStatus) validate() error {
	switch ds {
	case DepositoryUnverified, DepositoryVerified:
		return nil
	default:
		return fmt.Errorf("DepositoryStatus(%s) is invalid", ds)
	}
}

func (ds *DepositoryStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*ds = DepositoryStatus(strings.ToLower(s))
	if err := ds.validate(); err != nil {
		return err
	}
	return nil
}

// depositoryIdExists checks if a given DepositoryID belongs to the userId
func depositoryIdExists(userId string, id DepositoryID, repo depositoryRepository) bool {
	dep, err := repo.getUserDepository(id, userId)
	if err != nil || dep == nil {
		return false
	}
	return dep.ID == id
}

func addDepositoryRoutes(r *mux.Router, depositoryRepo depositoryRepository) {
	r.Methods("GET").Path("/depositories").HandlerFunc(getUserDepositories(depositoryRepo))
	r.Methods("POST").Path("/depositories").HandlerFunc(createUserDepository(depositoryRepo))

	r.Methods("GET").Path("/depositories/{depositoryId}").HandlerFunc(getUserDepository(depositoryRepo))
	r.Methods("PATCH").Path("/depositories/{depositoryId}").HandlerFunc(updateUserDepository(depositoryRepo))
	r.Methods("DELETE").Path("/depositories/{depositoryId}").HandlerFunc(deleteUserDepository(depositoryRepo))

	r.Methods("POST").Path("/depositories/{depositoryId}/micro-deposits").HandlerFunc(initiateMicroDeposits(depositoryRepo))
	r.Methods("POST").Path("/depositories/{depositoryId}/micro-deposits/confirm").HandlerFunc(confirmMicroDeposits(depositoryRepo))
}

// GET /depositories
// response: [ depository ]
func getUserDepositories(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserDepositories")
		if err != nil {
			return
		}

		userId := moovhttp.GetUserId(r)
		deposits, err := depositoryRepo.getUserDepositories(userId)
		if err != nil {
			internalError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(deposits); err != nil {
			internalError(w, err)
			return
		}
	}
}

func readDepositoryRequest(r *http.Request) (depositoryRequest, error) {
	var req depositoryRequest
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

// POST /depositories
// request: model w/o ID
// response: 201 w/ depository json
func createUserDepository(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserDepository")
		if err != nil {
			return
		}

		req, err := readDepositoryRequest(r)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		userId, now := moovhttp.GetUserId(r), time.Now()
		depository := &Depository{
			ID:            DepositoryID(nextID()),
			BankName:      req.BankName,
			Holder:        req.Holder,
			HolderType:    req.HolderType,
			Type:          req.Type,
			RoutingNumber: req.RoutingNumber,
			AccountNumber: req.AccountNumber,
			Status:        DepositoryUnverified,
			Metadata:      req.Metadata,
			Parent:        req.Parent,
			Created:       base.NewTime(now),
			Updated:       base.NewTime(now),
		}

		if err := depository.validate(); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		if err := depositoryRepo.upsertUserDepository(userId, depository); err != nil {
			internalError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(depository); err != nil {
			internalError(w, err)
			return
		}
	}
}

func getUserDepository(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserDepository")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		depository, err := depositoryRepo.getUserDepository(id, userId)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(depository); err != nil {
			internalError(w, err)
			return
		}
	}
}

func updateUserDepository(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "updateUserDepository")
		if err != nil {
			return
		}

		req, err := readDepositoryRequest(r)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		id, userId := getDepositoryId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		depository, err := depositoryRepo.getUserDepository(id, userId)
		if err != nil {
			internalError(w, err)
			return
		}
		if depository == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Update model
		if req.BankName != "" {
			depository.BankName = req.BankName
		}
		if req.Holder != "" {
			depository.Holder = req.Holder
		}
		if req.HolderType != "" {
			depository.HolderType = req.HolderType
		}
		if req.Type != "" {
			depository.Type = req.Type
		}
		if req.RoutingNumber != "" {
			depository.RoutingNumber = req.RoutingNumber
		}
		if req.AccountNumber != "" {
			depository.AccountNumber = req.AccountNumber
		}
		if req.Metadata != "" {
			depository.Metadata = req.Metadata
		}
		if !req.Parent.empty() {
			depository.Parent = req.Parent
		}
		depository.Updated = base.NewTime(time.Now())

		if err := depository.validate(); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		if err := depositoryRepo.upsertUserDepository(userId, depository); err != nil {
			internalError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(depository); err != nil {
			internalError(w, err)
			return
		}
	}
}

func deleteUserDepository(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserDepository")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := depositoryRepo.deleteUserDepository(id, userId); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
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

func markDepositoryVerified(repo depositoryRepository, id DepositoryID, userId string) error {
	dep, err := repo.getUserDepository(id, userId)
	if err != nil {
		return fmt.Errorf("markDepositoryVerified: depository %v (userId=%v): %v", id, userId, err)
	}
	dep.Status = DepositoryVerified
	return repo.upsertUserDepository(userId, dep)
}

type depositoryRepository interface {
	getUserDepositories(userId string) ([]*Depository, error)
	getUserDepository(id DepositoryID, userId string) (*Depository, error)

	upsertUserDepository(userId string, dep *Depository) error
	deleteUserDepository(id DepositoryID, userId string) error

	initiateMicroDeposits(id DepositoryID, userId string, amounts []Amount) error
	confirmMicroDeposits(id DepositoryID, userId string, amounts []Amount) error
}

type sqliteDepositoryRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *sqliteDepositoryRepo) close() error {
	return r.db.Close()
}

func (r *sqliteDepositoryRepo) getUserDepositories(userId string) ([]*Depository, error) {
	query := `select depository_id from depositories where user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	rows, err := stmt.Query(userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var depositoryIds []string
	for rows.Next() {
		var row string
		rows.Scan(&row)
		if row != "" {
			depositoryIds = append(depositoryIds, row)
		}
	}

	var depositories []*Depository
	for i := range depositoryIds {
		dep, err := r.getUserDepository(DepositoryID(depositoryIds[i]), userId)
		if err == nil && dep != nil && dep.BankName != "" {
			depositories = append(depositories, dep)
		}
	}
	return depositories, nil
}

// (depository_id primary key, user_id, bank_name, holder, holder_type, type, routing_number, account_number, status, metadata, parent, created_at, last_updated_at, deleted_at)

func (r *sqliteDepositoryRepo) getUserDepository(id DepositoryID, userId string) (*Depository, error) {
	query := `select depository_id, bank_name, holder, holder_type, type, routing_number, account_number, status, metadata, parent, created_at, last_updated_at
from depositories
where depository_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	row := stmt.QueryRow(id, userId)

	dep := &Depository{}
	var (
		created time.Time
		updated time.Time
	)
	err = row.Scan(&dep.ID, &dep.BankName, &dep.Holder, &dep.HolderType, &dep.Type, &dep.RoutingNumber, &dep.AccountNumber, &dep.Status, &dep.Metadata, &dep.Parent, &created, &updated)
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return nil, nil
		}
		return nil, err
	}
	dep.Created = base.NewTime(created)
	dep.Updated = base.NewTime(updated)
	if dep.ID == "" || dep.BankName == "" {
		return nil, nil // no records found
	}

	// TODO(adam): dep.validateStatus() ? (and other fields)

	return dep, nil
}

func (r *sqliteDepositoryRepo) upsertUserDepository(userId string, dep *Depository) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	now := base.NewTime(time.Now())
	if dep.Created.IsZero() {
		dep.Created = now
		dep.Updated = now
	}

	query := `insert or ignore into depositories (depository_id, user_id, bank_name, holder, holder_type, type, routing_number, account_number, status, metadata, parent, created_at, last_updated_at)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	res, err := stmt.Exec(dep.ID, userId, dep.BankName, dep.Holder, dep.HolderType, dep.Type, dep.RoutingNumber, dep.AccountNumber, dep.Status, dep.Metadata, dep.Parent, dep.Created.Time, dep.Updated.Time)
	if err != nil {
		return fmt.Errorf("problem upserting depository=%q, userId=%q: %v", dep.ID, userId, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		query = `update depositories
set bank_name = ?, holder = ?, holder_type = ?, type = ?, routing_number = ?,
account_number = ?, status = ?, metadata = ?, parent = ?, last_updated_at = ?
where depository_id = ? and user_id = ? and deleted_at is null`
		stmt, err := tx.Prepare(query)
		if err != nil {
			return err
		}

		_, err = stmt.Exec(
			dep.BankName, dep.Holder, dep.HolderType, dep.Type, dep.RoutingNumber,
			dep.AccountNumber, dep.Status, dep.Metadata, dep.Parent, time.Now(),
			dep.ID, userId)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *sqliteDepositoryRepo) deleteUserDepository(id DepositoryID, userId string) error {
	query := `update depositories set deleted_at = ? where depository_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	if _, err := stmt.Exec(time.Now(), id, userId); err != nil {
		return fmt.Errorf("error deleting depository_id=%q, user_id=%q: %v", id, userId, err)
	}
	return nil
}

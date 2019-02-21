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

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type CustomerID string

// Customer objects are organizations or people who receive an ACH Transfer from an Originator account.
//
// The API allows you to create, delete, and update your originators.
// You can retrieve individual originators as well as a list of all your originators. (Batch Header)
type Customer struct {
	// ID is a unique string representing this Customer.
	ID CustomerID `json:"id"`

	// Email address associated to Customer
	Email string `json:"email"` // TODO(adam): validate, public suffix list (PSL)

	// DefaultDepository is the Depository associated to this Customer.
	DefaultDepository DepositoryID `json:"defaultDepository"` // TODO(adam): validate

	// Status defines the current state of the Customer
	Status CustomerStatus `json:"status"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated base.Time `json:"updated"`
}

func (c *Customer) missingFields() error {
	if c.ID == "" {
		return errors.New("missing Customer.ID")
	}
	if c.Email == "" {
		return errors.New("missing Customer.Email")
	}
	if c.DefaultDepository == "" {
		return errors.New("missing Customer.DefaultDepository")
	}
	if c.Status == "" {
		return errors.New("missing Customer.Status")
	}
	return nil
}

// Validate checks the fields of Customer and returns any validation errors.
func (c *Customer) validate() error {
	if c == nil {
		return errors.New("nil Customer")
	}
	if err := c.missingFields(); err != nil {
		return err
	}

	// TODO(adam): validate email
	return c.Status.validate()
}

type CustomerStatus string

const (
	CustomerUnverified  CustomerStatus = "unverified"
	CustomerVerified    CustomerStatus = "verified"
	CustomerSuspended   CustomerStatus = "suspended"
	CustomerDeactivated CustomerStatus = "deactivated"
)

func (cs CustomerStatus) validate() error {
	switch cs {
	case CustomerUnverified, CustomerVerified, CustomerSuspended, CustomerDeactivated:
		return nil
	default:
		return fmt.Errorf("CustomerStatus(%s) is invalid", cs)
	}
}

func (cs *CustomerStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*cs = CustomerStatus(strings.ToLower(s))
	if err := cs.validate(); err != nil {
		return err
	}
	return nil
}

type customerRequest struct {
	Email             string       `json:"email,omitempty"`
	DefaultDepository DepositoryID `json:"defaultDepository,omitempty"`
	Metadata          string       `json:"metadata,omitempty"`
}

func (r customerRequest) missingFields() error {
	if r.Email == "" {
		return errors.New("missing customerRequest.Email")
	}
	if r.DefaultDepository.empty() {
		return errors.New("missing customerRequest.DefaultDepository")
	}
	return nil
}

func addCustomerRoutes(r *mux.Router, ofacClient OFACClient, customerRepo customerRepository, depositoryRepo depositoryRepository) {
	r.Methods("GET").Path("/customers").HandlerFunc(getUserCustomers(customerRepo))
	r.Methods("POST").Path("/customers").HandlerFunc(createUserCustomer(ofacClient, customerRepo, depositoryRepo))

	r.Methods("GET").Path("/customers/{customerId}").HandlerFunc(getUserCustomer(customerRepo))
	r.Methods("PATCH").Path("/customers/{customerId}").HandlerFunc(updateUserCustomer(customerRepo))
	r.Methods("DELETE").Path("/customers/{customerId}").HandlerFunc(deleteUserCustomer(customerRepo))
}

func getUserCustomers(customerRepo customerRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserCustomers")
		if err != nil {
			return
		}

		userId := moovhttp.GetUserId(r)
		customers, err := customerRepo.getUserCustomers(userId)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(customers); err != nil {
			internalError(w, err)
			return
		}
	}
}

func readCustomerRequest(r *http.Request) (customerRequest, error) {
	var req customerRequest
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

func createUserCustomer(ofacClient OFACClient, customerRepo customerRepository, depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserCustomer")
		if err != nil {
			return
		}

		req, err := readCustomerRequest(r)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		userId := moovhttp.GetUserId(r)
		if !depositoryIdExists(userId, req.DefaultDepository, depositoryRepo) {
			moovhttp.Problem(w, fmt.Errorf("Depository %s does not exist", req.DefaultDepository))
			return
		}

		// Create our customer
		customer := &Customer{
			ID:                CustomerID(nextID()),
			Email:             req.Email,
			DefaultDepository: req.DefaultDepository,
			Status:            CustomerUnverified,
			Metadata:          req.Metadata,
			Created:           base.NewTime(time.Now()),
		}
		if err := customer.validate(); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		// Check OFAC for customer/company data
		if err := rejectViaOFACMatch(logger, ofacClient, customer.Metadata, userId); err != nil {
			logger.Log("customers", err.Error(), "userId", userId)
			moovhttp.Problem(w, err)
			return
		}

		if err := customerRepo.upsertUserCustomer(userId, customer); err != nil {
			internalError(w, fmt.Errorf("creating customer=%q, user_id=%q", customer.ID, userId))
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(customer); err != nil {
			internalError(w, err)
			return
		}
	}
}

func getUserCustomer(customerRepo customerRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserCustomer")
		if err != nil {
			return
		}

		id, userId := getCustomerId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		customer, err := customerRepo.getUserCustomer(id, userId)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(customer); err != nil {
			internalError(w, err)
			return
		}
	}
}

func updateUserCustomer(customerRepo customerRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "updateUserCustomer")
		if err != nil {
			return
		}

		req, err := readCustomerRequest(r)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		id, userId := getCustomerId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		customer, err := customerRepo.getUserCustomer(id, userId)
		if err != nil {
			internalError(w, fmt.Errorf("problem getting customer=%q, user_id=%q", id, userId))
			return
		}
		if req.DefaultDepository != "" {
			customer.DefaultDepository = req.DefaultDepository
		}
		if req.Metadata != "" {
			customer.Metadata = req.Metadata
		}
		customer.Updated = base.NewTime(time.Now())

		if err := customer.validate(); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		// Perform update
		if err := customerRepo.upsertUserCustomer(userId, customer); err != nil {
			internalError(w, fmt.Errorf("updating customer=%q, user_id=%q", id, userId))
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(customer); err != nil {
			internalError(w, err)
			return
		}
	}
}

func deleteUserCustomer(customerRepo customerRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserCustomer")
		if err != nil {
			return
		}

		id, userId := getCustomerId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := customerRepo.deleteUserCustomer(id, userId); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
	}
}

// getCustomerId extracts the CustomerID from the incoming request.
func getCustomerId(r *http.Request) CustomerID {
	v := mux.Vars(r)
	id, ok := v["customerId"]
	if !ok {
		return CustomerID("")
	}
	return CustomerID(id)

}

type customerRepository interface {
	getUserCustomers(userId string) ([]*Customer, error)
	getUserCustomer(id CustomerID, userId string) (*Customer, error)

	upsertUserCustomer(userId string, cust *Customer) error
	deleteUserCustomer(id CustomerID, userId string) error
}

type sqliteCustomerRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *sqliteCustomerRepo) close() error {
	return r.db.Close()
}

func (r *sqliteCustomerRepo) getUserCustomers(userId string) ([]*Customer, error) {
	query := `select customer_id from customers where user_id = ? and deleted_at is null`
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

	var customerIds []string
	for rows.Next() {
		var row string
		rows.Scan(&row)
		if row != "" {
			customerIds = append(customerIds, row)
		}
	}

	var customers []*Customer
	for i := range customerIds {
		cust, err := r.getUserCustomer(CustomerID(customerIds[i]), userId)
		if err == nil && cust != nil && cust.Email != "" {
			customers = append(customers, cust)
		}
	}
	return customers, nil
}

func (r *sqliteCustomerRepo) getUserCustomer(id CustomerID, userId string) (*Customer, error) {
	query := `select customer_id, email, default_depository, status, metadata, created_at, last_updated_at
from customers
where customer_id = ?
and user_id = ?
and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userId)

	cust := &Customer{}
	err = row.Scan(&cust.ID, &cust.Email, &cust.DefaultDepository, &cust.Status, &cust.Metadata, &cust.Created.Time, &cust.Updated.Time)
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return nil, nil
		}
		return nil, err
	}
	if cust.ID == "" || cust.Email == "" {
		return nil, nil // no records found
	}

	return cust, nil
}

func (r *sqliteCustomerRepo) upsertUserCustomer(userId string, cust *Customer) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	now := time.Now()
	if cust.Created.IsZero() {
		cust.Created = base.NewTime(now)
		cust.Updated = base.NewTime(now)
	}

	query := `insert or ignore into customers (customer_id, user_id, email, default_depository, status, metadata, created_at, last_updated_at) values (?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var (
		created time.Time
		updated time.Time
	)
	res, err := stmt.Exec(cust.ID, userId, cust.Email, cust.DefaultDepository, cust.Status, cust.Metadata, &created, &updated)
	if err != nil {
		return fmt.Errorf("problem upserting customer=%q, userId=%q: %v", cust.ID, userId, err)
	}
	cust.Created = base.NewTime(created)
	cust.Updated = base.NewTime(updated)
	if n, _ := res.RowsAffected(); n == 0 {
		query = `update customers
set email = ?, default_depository = ?, status = ?, metadata = ?, last_updated_at = ?
where customer_id = ? and user_id = ? and deleted_at is null`
		stmt, err := tx.Prepare(query)
		if err != nil {
			return err
		}
		defer stmt.Close()

		_, err = stmt.Exec(cust.Email, cust.DefaultDepository, cust.Status, now, cust.ID, userId)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *sqliteCustomerRepo) deleteUserCustomer(id CustomerID, userId string) error {
	// TODO(adam): Should this just change the status to Deactivated?
	query := `update customers set deleted_at = ? where customer_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(time.Now(), id, userId); err != nil {
		return fmt.Errorf("error deleting customer_id=%q, user_id=%q: %v", id, userId, err)
	}
	return nil
}

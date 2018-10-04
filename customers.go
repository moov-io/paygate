// Copyright 2018 The Moov Authors
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

type CustomerID string

// Customer objects are organizations or people who receive an ACH Transfer from an Originator account.
//
// The API allows you to create, delete, and update your originators.
// You can retrieve individual originators as well as a list of all your originators. (Batch Header)
type Customer struct {
	ID                CustomerID     `json:"id"`
	Email             string         `json:"email"`
	DefaultDepository DepositoryID   `json:"defaultDepository"`
	Status            CustomerStatus `json:"status"`
	Metadata          string         `json:"metadata"`
	Created           time.Time      `json:"created"`
	Updated           time.Time      `json:"updated"`
}

type CustomerStatus string

const (
	CustomerUnverified  CustomerStatus = "Unverified"
	CustomerVerified                   = "Verified"
	CustomerSuspended                  = "Suspended"
	CustomerDeactivated                = "Deactivated"
)

func (cs *CustomerStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "unverified":
		*cs = CustomerUnverified
		return nil
	case "verified":
		*cs = CustomerVerified
		return nil
	case "suspended":
		*cs = CustomerSuspended
		return nil
	case "deactivated":
		*cs = CustomerDeactivated
		return nil
	}
	return fmt.Errorf("unknown CustomerStatus %q", s)
}

type customerRequest struct { // TODO(adam): we need to update the openapi docs
	Email             string       `json:"email,omitempty"`
	DefaultDepository DepositoryID `json:"defaultDepository,omitempty"`
	Metadata          string       `json:"metadata,omitempty"`
}

func (r customerRequest) missingFields() bool {
	return r.Email == "" || r.DefaultDepository.empty()
}

func addCustomerRoutes(r *mux.Router, customerRepo customerRepository) {
	r.Methods("GET").Path("/customers").HandlerFunc(getUserCustomers(customerRepo))
	r.Methods("POST").Path("/customers").HandlerFunc(createUserCustomer(customerRepo))

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

		userId := getUserId(r)
		customers, err := customerRepo.getUserCustomers(userId)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(customers); err != nil {
			internalError(w, err, "getUserCustomers")
			return
		}
	}
}

func createUserCustomer(customerRepo customerRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserCustomer")
		if err != nil {
			return
		}

		bs, err := read(r.Body)
		if err != nil {
			encodeError(w, err)
			return
		}
		var req customerRequest
		if err := json.Unmarshal(bs, &req); err != nil {
			encodeError(w, err)
			return
		}

		if req.missingFields() {
			encodeError(w, errMissingRequiredJson)
			return
		}

		userId := getUserId(r)
		customer, err := customerRepo.createUserCustomer(userId, req)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(customer); err != nil {
			internalError(w, err, "createUserCustomer")
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

		id, userId := getCustomerId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		customer, err := customerRepo.getUserCustomer(id, userId)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(customer); err != nil {
			internalError(w, err, "getUserCustomer")
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

		bs, err := read(r.Body)
		if err != nil {
			encodeError(w, err)
			return
		}
		var req customerRequest
		if err := json.Unmarshal(bs, &req); err != nil {
			encodeError(w, err)
			return
		}

		id, userId := getCustomerId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		customer, err := customerRepo.updateUserCustomer(id, userId, req)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(customer); err != nil {
			internalError(w, err, "createUserCustomer")
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

		id, userId := getCustomerId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := customerRepo.deleteUserCustomer(id, userId); err != nil {
			encodeError(w, err)
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

	createUserCustomer(userId string, req customerRequest) (*Customer, error)
	updateUserCustomer(id CustomerID, userId string, req customerRequest) (*Customer, error)
	deleteUserCustomer(id CustomerID, userId string) error
}

type memCustomerRepo struct{}

func (m memCustomerRepo) getUserCustomers(userId string) ([]*Customer, error) {
	cust, err := m.getUserCustomer(CustomerID(nextID()), userId)
	if err != nil {
		return nil, err
	}
	return []*Customer{cust}, nil
}

func (memCustomerRepo) getUserCustomer(id CustomerID, userId string) (*Customer, error) {
	return &Customer{
		ID:                id,
		Email:             "foo@moov.io",
		DefaultDepository: DepositoryID(nextID()),
		Status:            CustomerVerified,
		Created:           time.Now(),
	}, nil
}

func (m memCustomerRepo) createUserCustomer(userId string, req customerRequest) (*Customer, error) {
	return m.getUserCustomer(CustomerID(nextID()), userId)
}

func (m memCustomerRepo) updateUserCustomer(id CustomerID, userId string, req customerRequest) (*Customer, error) {
	return m.getUserCustomer(id, userId)
}

func (memCustomerRepo) deleteUserCustomer(id CustomerID, userId string) error {
	return nil
}

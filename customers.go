// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
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
	Created           *time.Time     `json:"createdp"`
	Updated           *time.Time     `json:"updated"`
}

type CustomerStatus string

const (
	CustomerUnverified  CustomerStatus = "Unverified"
	CustomerVerified                   = "Verified"
	CustomerSuspended                  = "Suspended"
	CustomerDeactivated                = "Deactivated"
)

func (cs *CustomerStatus) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("CustomerStatus must be a quoted string")
	}

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

type customerRequest struct {
	Email             string         `json:"email,omitempty"`
	DefaultDepository DepositoryID   `json:"defaultDepository,omitempty"`
	Status            CustomerStatus `json:"status,omitempty"`
	Metadata          string         `json:"metadata,omitempty"`
	Created           *time.Time     `json:"createdp,omitempty"`
	Updated           *time.Time     `json:"updated,omitempty"`
}

func addCustomerRoutes(r *mux.Router) {

}

// GET /customers
// [
//   ...
// ]
//
// POST /customers
// input
// {
// 	"id": "feb492e6",
// 	"email": "string",
// 	"defaultDepository": "0c5e215c",
// 	"status": "unverified",
// 	"metadata": "Authorized for re-occurring WEB",
// 	"created": "2018-09-27T17:05:45.056Z",
// 	"updated": "2018-09-27T17:05:45.056Z"
// }
// output
// 201: {
//         "id": "feb492e6",
// 	"email": "string",
// 	"defaultDepository": "0c5e215c",
// 	"status": "unverified",
// 	"metadata": "Authorized for re-occurring WEB",
// 	"created": "2018-09-27T17:05:45.111Z",
// 	"updated": "2018-09-27T17:05:45.111Z"
// }
//
// GET /customers/{id}
//
// PATCH /customers/{id}
// input
// {
// 	"id": "feb492e6",
// 	"email": "string",
// 	"defaultDepository": "0c5e215c",
// 	"status": "unverified",
// 	"metadata": "Authorized for re-occurring WEB",
// 	"created": "2018-09-27T17:07:17.698Z",
// 	"updated": "2018-09-27T17:07:17.698Z"
// }
// output
// 201: customer object
//
// DELETE /customers/{id}

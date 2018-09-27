// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"time"

	"github.com/gorilla/mux"
)

type CustomerID string

type Customer struct {
	ID                CustomerID     `json:"id"`
	Email             string         `json:"email"`
	DefaultDepository DepositoryID   `json:"defaultDepository"`
	Status            CustomerStatus `json:"status"`
	Metadata          string         `json:"metadata"`
	Created           *time.Time     `json:"createdp"`
	Updated           *time.Time     `json:"updated"`
}

type CustomerStatus int

func (cs CustomerStatus) String() string {
	switch cs {
	case CustomerUnverified:
		return "unverified"
	case CustomerVerified:
		return "verified"
	case CustomerSuspended:
		return "suspended"
	case CustomerDeactivated:
		return "deactivated"
	}
	return "unknown"
}

const (
	CustomerUnverified CustomerStatus = iota
	CustomerVerified
	CustomerSuspended
	CustomerDeactivated
)

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

// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"time"

	"github.com/gorilla/mux"
)

type DepositoryID string

type Depository struct {
	ID            DepositoryID
	BankName      string
	Holder        string
	HolderType    HolderType
	Type          AccountType
	RoutingNumber string
	AccountNumber string
	Status        DepositoryStatus
	Metadata      string
	Parent        *DepositoryID
	Created       *time.Time
	Updated       *time.Time
}

type HolderType int

const (
	Individual HolderType = iota
	Business
)

type DepositoryStatus int

const (
	DepositoryUnverified DepositoryStatus = iota
	DepositoryVerified
)

func addDepositoryRoutes(r *mux.Router) {

}

// GET /depositories
// [
// 	{
// 		"id": "0c5e215c",
// 		"bankName": "MVB Bank, Inc.",
// 		"holder": "My Company,llc or Wade Arnold",
// 		"holderType": "individual",
// 		"type": "checking",
// 		"routingNumber": "051504597",
// 		"accountNumber": "0001027028",
// 		"status": "unverified",
// 		"metadata": "Payroll",
// 		"parent": "feb492e6",
// 		"created": "2018-09-27T17:08:12.191Z",
// 		"updated": "2018-09-27T17:08:12.191Z"
// 	}
// ]
//
// POST /depositories
// {
// 	"id": "0c5e215c",
// 	"bankName": "MVB Bank, Inc.",
// 	"holder": "My Company,llc or Wade Arnold",
// 	"holderType": "individual",
// 	"type": "checking",
// 	"routingNumber": "051504597",
// 	"accountNumber": "0001027028",
// 	"status": "unverified",
// 	"metadata": "Payroll",
// 	"parent": "feb492e6",
// 	"created": "2018-09-27T17:08:30.757Z",
// 	"updated": "2018-09-27T17:08:30.757Z"
// }
//
// output
// 201: {
//      "id": "0c5e215c",
// 	"bankName": "MVB Bank, Inc.",
// 	"holder": "My Company,llc or Wade Arnold",
// 	"holderType": "individual",
// 	"type": "checking",
// 	"routingNumber": "051504597",
// 	"accountNumber": "0001027028",
// 	"status": "unverified",
// 	"metadata": "Payroll",
// 	"parent": "feb492e6",
// 	"created": "2018-09-27T17:08:30.841Z",
// 	"updated": "2018-09-27T17:08:30.841Z"
// }
//
// GET /depositories/{id}
// PATCH /depositories/{id}
// DELETE /depositories/{id}
//
// POST /depositories/{id}/micro-deposits
// 200 - Micro deposits verified
// 201 - Micro deposits initiated
// 400 - Invalid Amounts
// 404 - A depository with the specified ID was not found.
// 409 - Too many attempts. Bank already verified.
//

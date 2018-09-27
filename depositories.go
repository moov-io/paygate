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

type DepositoryID string

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
	Created       *time.Time       `json:"created"`
	Updated       *time.Time       `json:"updated"`
}

type HolderType string

const (
	Individual HolderType = "Individual"
	Business              = "Business"
)

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

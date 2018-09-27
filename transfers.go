// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"time"

	"github.com/gorilla/mux"
)

type TransferID string

type Transfer struct {
	ID                     TransferID     `json:"id"`
	Type                   TransferType   `json:"transferType"`
	Amount                 Amount         `json:"amount"`
	Originator             OriginatorID   `json:"originator"`
	OriginatorDepository   DepositoryID   `json:"originatorDepository"`
	Customer               CustomerID     `json:"customer"`
	CustomerDepository     DepositoryID   `json:"customerDepository"`
	Description            string         `json:"description"`
	StandardEntryClassCode string         `json:"standardEntryClassCode"`
	Status                 TransferStatus `json:"status"`
	SameDay                bool           `json:"sameDay"`
	Created                *time.Time     `json:"created"`

	WEBDetail WEBDetail `json:"WEBDetail,omitempty"`
}

type TransferType string

const (
	PushTransfer TransferType = "push"
)

type TransferStatus string

const (
	TransferCanceled  TransferStatus = "canceled"
	TransferFailed                   = "failed"
	TransferPending                  = "pending"
	TransferProcessed                = "processed"
	TransferReclaimed                = "reclaimed"
)

type WEBDetail struct { // TODO(adam): lowercase names?
	PaymentType WEBPaymentType `json:"PaymentType"`
}

type WEBPaymentType string

const (
	WEBSingle      WEBPaymentType = "single"
	WEBReoccurring                = "reoccurring"
)

func addTransfersRoute(r *mux.Router) {

}

// GET /transfers
// [
// 	{
// 		"id": "33164ac6",
// 		"type": "push",
// 		"amount": "USD 99.99",
// 		"originator": "724b6abe",
// 		"originatorDepository": "59276ce4",
// 		"customer": "feb492e6",
// 		"customerDepository": "dad7ddfb",
// 		"description": "Loan Pay",
// 		"standardEntryClassCode": "WEB",
// 		"status": "processed",
// 		"sameDay": false,
// 		"created": "2018-09-27T17:10:48.509Z",
// 		"WEBDetail": {
// 			"PaymentType": "single"
// 		}
// 	}
// ]
//
// POST /transfers
// input
// {
// 	"id": "33164ac6",
// 	"type": "push",
// 	"amount": "USD 99.99",
// 	"originator": "724b6abe",
// 	"originatorDepository": "59276ce4",
// 	"customer": "feb492e6",
// 	"customerDepository": "dad7ddfb",
// 	"description": "Loan Pay",
// 	"standardEntryClassCode": "WEB",
// 	"status": "processed",
// 	"sameDay": false,
// 	"created": "2018-09-27T17:11:06.192Z",
// 	"WEBDetail": {
// 		"PaymentType": "single"
// 	}
// }
//
// POST /transfers/batch
// input: [ .. ]
//
// GET /transfers/{id}
// DELETE /transfers/{id}
//
// POST /transfers/{id}/failed
// 200 - no errors
// 400 - errors, check json
//
// POST /transfers/{id}/files
//
// GET /transfers/{id}/events

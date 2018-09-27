// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"time"

	"github.com/gorilla/mux"
)

type OriginatorID string

// Originator objects are an organization or person that initiates an ACH Transfer to a Customer account either as a debit or credit. The API allows you to create, delete, and update your originators. You can retrieve individual originators as well as a list of all your originators. (Batch Header)
type Originator struct {
	// ID is a globally unique identifier
	ID OriginatorID `json:"id"`
	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository string `json:"defaultDepository"`
	// Identification is a number by which the customer is known to the originator
	Identification string `json:"identification"`
	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`
	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created *time.Time `json:"created"`
	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated *time.Time `json:"updated"`
}

func addOriginatorRoutes(r *mux.Router) {

}

// GET		/originators
// [
// 	{
// 		"id": "724b6abe",
// 		"defaultDepository": "0c0c3412",
// 		"identification": null,
// 		"metadata": "Primary payment account",
// 		"created": "2018-09-27T17:02:44.931Z",
// 		"updated": "2018-09-27T17:02:44.931Z"
// 	}
// ]
//
// POST		/originators
// input
// {
// 	"id": "724b6abe",
// 	"defaultDepository": "0c0c3412",
// 	"identification": null,
// 	"metadata": "Primary payment account",
// 	"created": "2018-09-27T17:03:12.175Z",
// 	"updated": "2018-09-27T17:03:12.175Z"
// }
// output
// {
// 	"id": "724b6abe",
// 	"defaultDepository": "0c0c3412",
// 	"identification": null,
// 	"metadata": "Primary payment account",
// 	"created": "2018-09-27T17:03:12.225Z",
// 	"updated": "2018-09-27T17:03:12.225Z"
// }
//
// GET		/originators/{originatorId}
// (originator json)
// PATCH	/originators/{originatorId}
// input
// {
// 	"id": "724b6abe",
// 	"defaultDepository": "0c0c3412",
// 	"identification": null,
// 	"metadata": "Primary payment account",
// 	"created": "2018-09-27T17:04:08.684Z",
// 	"updated": "2018-09-27T17:04:08.684Z"
// }
// output
// '201': {
// 	"id": "724b6abe",
// 	"defaultDepository": "0c0c3412",
// 	"identification": null,
// 	"metadata": "Primary payment account",
// 	"created": "2018-09-27T17:04:08.754Z",
// 	"updated": "2018-09-27T17:04:08.754Z"
// }
// DELETE	/originators/{originatorId}
// 200

// r.Methods("POST").Path("/originators/").Handler(httptransport.NewServer(
// 	e.NewOriginatorEndpoint,
// 	decodeNewOriginatorRequest,
// 	encodeResponse,
// 	options...,
// ))
// r.Methods("GET").Path("/originators/").Handler(httptransport.NewServer(
// 	e.ListOriginatorsEndpoint,
// 	decodeListOriginatorsRequest,
// 	encodeResponse,
// 	options...,
// ))
// r.Methods("GET").Path("/originators/{id}").Handler(httptransport.NewServer(
// 	e.LoadOriginatorEndpoint,
// 	decodeLoadOriginatorRequest,
// 	encodeResponse,
// 	options...,
// ))

// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

type OriginatorID string

// Originator objects are an organization or person that initiates an ACH Transfer to a Customer account either as a debit or credit. The API allows you to create, delete, and update your originators. You can retrieve individual originators as well as a list of all your originators. (Batch Header)
type Originator struct {
	// ID is a globally unique identifier
	ID OriginatorID
	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository string
	// Identification is a number by which the customer is known to the originator
	Identification string
	// MetaData provides additional data to be used for display and search only
	MetaData string
	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created string
	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated string
}

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

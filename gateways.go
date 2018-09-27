// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"time"

	"github.com/gorilla/mux"
)

type GatewayID string

type Gateway struct {
	ID              GatewayID  `json:"id"`
	Origin          string     `json:"origin"`
	OriginName      string     `json:"originName"`
	Destination     string     `json:"destination"`
	DestinationName string     `json:"destinationName"`
	Created         *time.Time `json:"created"`
}

func addGatewayRoutes(r *mux.Router) {

}

// GET /gateways
// [
// 	{
// 		"id": "string",
// 		"origin": 99991234,
// 		"originName": "My Bank Name",
// 		"destination": 69100013,
// 		"destinationName": "Federal Reserve Bank",
// 		"created": "2018-09-27T17:13:44.505Z"
// 	}
// ]
//
// POST /gateways
// {
// 	"origin": 99991234,
// 	"originName": "My Bank Name",
// 	"destination": 69100013,
// 	"destinationName": "Federal Reserve Bank",
// 	"created": "2018-09-27T17:13:44.505Z"
// }

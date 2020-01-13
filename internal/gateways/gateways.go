// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gateways

import (
	"errors"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
)

type GatewayID string

type Gateway struct {
	// ID is a unique string representing this Gateway.
	ID GatewayID `json:"id"`

	// Origin is an ABA routing number
	Origin string `json:"origin"`

	// OriginName is the legal name associated with the origin routing number.
	OriginName string `json:"originName"`

	// Destination is an ABA routing number
	Destination string `json:"destination"`

	// DestinationName is the legal name associated with the destination routing number.
	DestinationName string `json:"destinationName"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`
}

func (g *Gateway) validate() error {
	if g == nil {
		return errors.New("nil Gateway")
	}

	// Origin
	if err := ach.CheckRoutingNumber(g.Origin); err != nil {
		return err
	}
	if g.OriginName == "" {
		return errors.New("missing Gateway.OriginName")
	}

	// Destination
	if err := ach.CheckRoutingNumber(g.Destination); err != nil {
		return err
	}
	if g.DestinationName == "" {
		return errors.New("missing Gateway.DestinationName")
	}
	return nil
}

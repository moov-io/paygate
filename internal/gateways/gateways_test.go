// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gateways

import (
	"testing"
)

func TestGateway__validate(t *testing.T) {
	var g *Gateway

	if err := g.validate(); err == nil {
		t.Error("expected error")
	}

	g = &Gateway{
		Origin: "123",
	}
	if err := g.validate(); err == nil {
		t.Error("expected error")
	}

	g.Origin = "987654320"
	if err := g.validate(); err == nil {
		t.Error("expected error")
	}

	g.OriginName = "Moov Bank"
	if err := g.validate(); err == nil {
		t.Error("expected error")
	}

	g.Destination = "1234"
	if err := g.validate(); err == nil {
		t.Error("expected error")
	}

	g.Destination = "123456780"
	if err := g.validate(); err == nil {
		t.Error("expected error")
	}

	g.DestinationName = "Other Bank"
	if err := g.validate(); err != nil {
		t.Errorf("error=%v", err)
	}
}

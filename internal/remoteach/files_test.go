// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package remoteach

import (
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
)

func TestTransfers__ABA(t *testing.T) {
	routingNumber := "231380104"
	if v := ABA8(routingNumber); v != "23138010" {
		t.Errorf("got %s", v)
	}
	if v := ABACheckDigit(routingNumber); v != "4" {
		t.Errorf("got %s", v)
	}

	// 10 digit from ACH server
	if v := ABA8("0123456789"); v != "12345678" {
		t.Errorf("got %s", v)
	}
	if v := ABACheckDigit("0123456789"); v != "9" {
		t.Errorf("got %s", v)
	}
}

func TestTransfers__CreateTraceNumber(t *testing.T) {
	if v := CreateTraceNumber("121042882"); v == "" {
		t.Error("empty trace number")
	}
}

func TestTransfers__ConstructFile(t *testing.T) {
	// The fields on each struct are minimized to help throttle this file's size
	receiverDep := &model.Depository{
		BankName:      "foo bank",
		RoutingNumber: "121042882",
	}
	receiver := &model.Receiver{Status: model.ReceiverVerified}
	origDep := &model.Depository{
		BankName:      "foo bank",
		RoutingNumber: "231380104",
	}
	orig := &model.Originator{}
	transfer := &model.Transfer{
		Type:                   model.PushTransfer,
		Status:                 model.TransferPending,
		StandardEntryClassCode: "AAA", // invalid
		UserID:                 base.ID(),
	}

	file, err := ConstructFile("", "", transfer, receiver, receiverDep, orig, origDep)
	if err == nil || file != nil {
		t.Fatalf("expected error, got file=%#v", file)
	}
	if !strings.Contains(err.Error(), "unsupported SEC code: AAA") {
		t.Errorf("unexpected error: %v", err)
	}
}

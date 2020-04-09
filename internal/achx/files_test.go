// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
)

func TestTransfers__ConstructFile(t *testing.T) {
	// The fields on each struct are minimized to help throttle this file's size
	gateway := &model.Gateway{
		ID:              model.GatewayID(base.ID()),
		Origin:          "987654320",
		OriginName:      "My Bank",
		Destination:     "123456780",
		DestinationName: "Their Bank",
	}
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

	file, err := ConstructFile("", "", gateway, transfer, receiver, receiverDep, orig, origDep)
	if err == nil || file != nil {
		t.Fatalf("expected error, got file=%#v", file)
	}
	if !strings.Contains(err.Error(), "unsupported SEC code: AAA") {
		t.Errorf("unexpected error: %v", err)
	}
}

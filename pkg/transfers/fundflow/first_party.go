// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package fundflow

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/achx"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"

	"github.com/go-kit/kit/log"
)

// FirstPerson returns a Strategy for fund flows where PayGate runs as an ACH originator
// at an FI. This implies funds move in one direction from the FI -- either in or out.
//
// Outgoing credits are debited from the account at the FI without delay and the credits
// are posted after the RDFI receives the file.
//
// Debiting the remote account means we'll credit our account, but typically hold
// those funds for a settlement period.
//
// These transfers involve one file with an optional return from the RDFI which should trigger
// a reversal in the accounting ledger.
type FirstParty struct {
	cfg    config.ODFI
	logger log.Logger
}

func NewFirstPerson(logger log.Logger, cfg config.ODFI) Strategy {
	return &FirstParty{
		cfg:    cfg,
		logger: logger,
	}
}

func (fp *FirstParty) Originate(companyID string, xfer *client.Transfer, src Source, dst Destination) ([]*ach.File, error) {
	source := achx.Source{
		Customer: src.Customer,
		Account:  src.Account,
	}
	destination := achx.Destination{
		Customer:      dst.Customer,
		Account:       dst.Account,
		AccountNumber: dst.AccountNumber,
	}

	file, err := achx.ConstrctFile(xfer.TransferID, fp.cfg, companyID, xfer, source, destination)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: transferID=%s: %v", xfer.TransferID, err)
	}
	return []*ach.File{file}, err
}

func (fp *FirstParty) HandleReturn(returned *ach.File, xfer *client.Transfer) ([]*ach.File, error) {
	return nil, nil
}

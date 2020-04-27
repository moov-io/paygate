// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"fmt"
	"time"

	"github.com/moov-io/ach"
	customers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
)

type Source struct {
	Customer customers.Customer
	Account  customers.Account
}

type Destination struct {
	Customer customers.Customer
	Account  customers.Account
}

func ConstrctFile(id string, odfi config.ODFI, xfer client.Transfer, source Source, destination Destination) (*ach.File, error) {
	file, now := ach.NewFile(), time.Now()
	file.ID = id
	file.Control = ach.NewFileControl()

	// File Header
	file.Header.ID = id
	file.Header.ImmediateOrigin = odfi.Gateway.Origin
	file.Header.ImmediateOriginName = odfi.Gateway.OriginName
	file.Header.ImmediateDestination = odfi.Gateway.Destination
	file.Header.ImmediateDestinationName = odfi.Gateway.DestinationName
	file.Header.FileCreationDate = now.Format("060102") // YYMMDD
	file.Header.FileCreationTime = now.Format("1504")   // HHMM

	// Right now we only support creating PPD files
	batch, err := createPPDBatch(id, odfi, xfer, source, destination)
	if err != nil {
		return nil, fmt.Errorf("constructACHFile: PPD: %v", err)
	}
	file.AddBatch(batch)

	return file, file.Validate()
}

// func determineTransactionCode(t *model.Transfer, origDep *model.Depository) int {
// 	switch {
// 	case t == nil || origDep == nil:
// 		return 0 // invalid, so we error
// 	case strings.EqualFold(t.StandardEntryClassCode, ach.TEL):
// 		// Per NACHA guidelines:
// 		//   "TEL Entries may only be used for debit transactions only."
// 		if origDep.Type == model.Checking {
// 			return ach.CheckingDebit
// 		}
// 		return ach.SavingsDebit
// 	default:
// 		if origDep.Type == model.Checking {
// 			if t.Type == model.PushTransfer {
// 				return ach.CheckingCredit
// 			}
// 			return ach.CheckingDebit
// 		} else { // Savings
// 			if t.Type == model.PushTransfer {
// 				return ach.SavingsCredit
// 			}
// 			return ach.SavingsDebit
// 		}
// 	}
// 	// Credit (deposit) to checking account ‘22’
// 	// Prenote for credit to checking account ‘23’
// 	// Debit (withdrawal) to checking account ‘27’
// 	// Prenote for debit to checking account ‘28’
// 	// Credit to savings account ‘32’
// 	// Prenote for credit to savings account ‘33’
// 	// Debit to savings account ‘37’
// 	// Prenote for debit to savings account ‘38’
// }

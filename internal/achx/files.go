// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"fmt"
	"strings"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
)

// ConstructFile will take in a Transfer and metadata to build an ACH file which can be submitted against an ACH instance.
func ConstructFile(
	id string,
	gateway *model.Gateway, // TODO(adam): so, this probably needs to be a pre-upload transform
	transfer *model.Transfer,
	orig *model.Originator,
	origDep *model.Depository,
	receiver *model.Receiver,
	receiverDep *model.Depository,
) (*ach.File, error) {
	// Create our ACH file
	file, now := ach.NewFile(), time.Now()
	file.ID = id
	file.Control = ach.NewFileControl()

	// File Header
	file.Header.ID = id
	file.Header.ImmediateOrigin = gateway.Origin
	file.Header.ImmediateOriginName = gateway.OriginName
	file.Header.ImmediateDestination = gateway.Destination
	file.Header.ImmediateDestinationName = gateway.DestinationName
	file.Header.FileCreationDate = now.Format("060102") // YYMMDD
	file.Header.FileCreationTime = now.Format("1504")   // HHMM

	// Add batch to our ACH file
	switch transfer.StandardEntryClassCode {
	case ach.CCD: // TODO(adam): Do we need to handle ACK also?
		batch, err := createCCDBatch(id, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddBatch(batch)
	case ach.IAT:
		batch, err := createIATBatch(id, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddIATBatch(*batch)
	case ach.PPD:
		batch, err := createPPDBatch(id, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddBatch(batch)
	case ach.TEL:
		batch, err := createTELBatch(id, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddBatch(batch)
	case ach.WEB:
		batch, err := createWEBBatch(id, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddBatch(batch)
	default:
		return nil, fmt.Errorf("unsupported SEC code: %s", transfer.StandardEntryClassCode)
	}
	return file, nil
}

func determineServiceClassCode(t *model.Transfer) int {
	if strings.EqualFold(t.StandardEntryClassCode, ach.TEL) {
		return ach.DebitsOnly
	}
	if t.Type == model.PushTransfer {
		return ach.CreditsOnly
	}
	return ach.DebitsOnly
}

func determineTransactionCode(t *model.Transfer, origDep *model.Depository) int {
	switch {
	case t == nil || origDep == nil:
		return 0 // invalid, so we error
	case strings.EqualFold(t.StandardEntryClassCode, ach.TEL):
		// Per NACHA guidelines:
		//   "TEL Entries may only be used for debit transactions only."
		if origDep.Type == model.Checking {
			return ach.CheckingDebit
		}
		return ach.SavingsDebit
	default:
		if origDep.Type == model.Checking {
			if t.Type == model.PushTransfer {
				return ach.CheckingCredit
			}
			return ach.CheckingDebit
		} else { // Savings
			if t.Type == model.PushTransfer {
				return ach.SavingsCredit
			}
			return ach.SavingsDebit
		}
	}
}

func createIdentificationNumber() string {
	return base.ID()[:15]
}

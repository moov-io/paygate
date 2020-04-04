// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package remoteach

import (
	"fmt"
	"strings"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/achclient"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

// CheckFile calls out to our ACH service to build and validate the ACH file,
// "build" involves the ACH service computing some file/batch level totals and checksums.
func CheckFile(logger log.Logger, client *achclient.ACH, fileID string, userID id.User) error {
	// We don't care about the resposne, just the side-effect build tabulations.
	if _, err := client.GetFileContents(fileID); err != nil && logger != nil {
		logger.Log("transfers", fmt.Sprintf("responder.XUserID=%s fileID=%s err=%v", userID, fileID, err))
	}
	// ValidateFile will return specific file-level information about what's wrong.
	return client.ValidateFile(fileID)
}

// ConstructFile will take in a Transfer and metadata to build an ACH file which can be submitted against an ACH instance.
func ConstructFile(
	id, idempotencyKey string,
	gateway *model.Gateway,
	transfer *model.Transfer,
	receiver *model.Receiver,
	receiverDep *model.Depository,
	orig *model.Originator,
	origDep *model.Depository,
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
	if t.Type == model.PushTransfer {
		return ach.CreditsOnly
	}
	return ach.DebitsOnly
}

func determineTransactionCode(t *model.Transfer, origDep *model.Depository) int {
	switch {
	case t == nil:
		return 0 // invalid, so we error
	case strings.EqualFold(t.StandardEntryClassCode, ach.TEL):
		if origDep.Type == model.Checking {
			return ach.CheckingDebit // Debit (withdrawal) to checking account ‘27’
		}
		return ach.SavingsDebit // Debit to savings account ‘37’
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
	// Credit (deposit) to checking account ‘22’
	// Prenote for credit to checking account ‘23’
	// Debit (withdrawal) to checking account ‘27’
	// Prenote for debit to checking account ‘28’
	// Credit to savings account ‘32’
	// Prenote for credit to savings account ‘33’
	// Debit to savings account ‘37’
	// Prenote for debit to savings account ‘38’
}

func createIdentificationNumber() string {
	return base.ID()[:15]
}

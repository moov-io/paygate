// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package remoteach

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/achclient"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

var (
	traceNumberSource = rand.NewSource(time.Now().Unix())
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
func ConstructFile(id, idempotencyKey string, transfer *model.Transfer, receiver *model.Receiver, receiverDep *model.Depository, orig *model.Originator, origDep *model.Depository) (*ach.File, error) {
	// TODO(adam): KYC (via Customers) is needed before we validate / reject Receivers
	// TODO(adam): why are these checks in this method?
	if transfer.Type == model.PullTransfer && receiver.Status != model.ReceiverVerified {
		// TODO(adam): "additional checks" - check Receiver.Status ???
		// https://github.com/moov-io/paygate/issues/18#issuecomment-432066045
		return nil, fmt.Errorf("receiver_id=%s is not Verified user_id=%s", receiver.ID, transfer.UserID)
	}
	if transfer.Status != model.TransferPending {
		return nil, fmt.Errorf("transfer_id=%s is not Pending (status=%s)", transfer.ID, transfer.Status)
	}

	// Create our ACH file
	file, now := ach.NewFile(), time.Now()
	file.ID = id
	file.Control = ach.NewFileControl()

	// File Header
	file.Header.ID = id
	file.Header.ImmediateOrigin = origDep.RoutingNumber
	file.Header.ImmediateOriginName = origDep.BankName
	file.Header.ImmediateDestination = receiverDep.RoutingNumber
	file.Header.ImmediateDestinationName = receiverDep.BankName
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

func createTraceNumber(odfiRoutingNumber string) string {
	v := fmt.Sprintf("%s%d", aba8(odfiRoutingNumber), traceNumberSource.Int63())
	if utf8.RuneCountInString(v) > 15 {
		return v[:15]
	}
	return v
}

// aba8 returns the first 8 digits of an ABA routing number.
// If the input is invalid then an empty string is returned.
func aba8(rtn string) string {
	if n := utf8.RuneCountInString(rtn); n == 10 {
		return rtn[1:9] // ACH server will prefix with space, 0, or 1
	}
	if n := utf8.RuneCountInString(rtn); n != 8 && n != 9 {
		return ""
	}
	return rtn[:8]
}

// abaCheckDigit returns the last digit of an ABA routing number.
// If the input is invalid then an empty string is returned.
func abaCheckDigit(rtn string) string {
	if n := utf8.RuneCountInString(rtn); n == 10 {
		return rtn[9:] // ACH server will prefix with space, 0, or 1
	}
	if n := utf8.RuneCountInString(rtn); n != 8 && n != 9 {
		return ""
	}
	return rtn[8:9]
}

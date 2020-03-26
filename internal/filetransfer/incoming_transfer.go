// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
)

// TODO(adam): handle incoming file as a transfer (reconcile against Depositories, create Transfer row, NSF/return, etc..)

func (c *Controller) handleIncomingTransfer(req *periodicFileOperationsRequest, file *ach.File, filename string) error {
	c.logger.Log("handleIncomingTransfer", fmt.Sprintf("incoming ACH file %s", filename), "userID", req.userID, "requestID", req.requestID)

	for i := range file.Batches {
		// Skip this file (after creating a Correction and/or Return)
		if err := easilyRejectableFile(req, file.Batches[i], filename); err != nil {
			header := file.Batches[i].GetHeader()
			c.logger.Log(
				"handleIncomingTransfer", fmt.Sprintf("skipping file=%s batch=%d: %v", filename, header.BatchNumber, err),
				"userID", req.userID, "requestID", req.requestID)
			return nil
		}

		// Process each entry as if it's a Transfer
		entries := file.Batches[i].GetEntries()
		for j := range entries {
			// Section 3.1.2 allows an RDFI to rely on account number in the EntryDetail to post transactions
			dep, err := c.depRepo.LookupDepositoryForIncoming(file.Header.ImmediateDestination, entries[j].DFIAccountNumber, entries[j].IndividualName)
			if err != nil {
				c.logger.Log(
					"handleIncomingTransfer", fmt.Sprintf("unable to find depository: %v", err),
					"userID", req.userID, "requestID", req.requestID)
				continue
			}
			if dep == nil || dep.ID == "" {
				c.logger.Log(
					"handleIncomingTransfer", fmt.Sprintf("depository not found traceNumber=%s", entries[j].TraceNumber),
					"userID", req.userID, "requestID", req.requestID)
				continue
			}

			xfer, err := createTransferFromEntry(dep.UserID, file.Header, file.Batches[i].GetHeader(), entries[j])
			if err != nil {
				fmt.Printf("error=%v\n", err)
			}
		}
	}

	return nil
}

func easilyRejectableFile(req *periodicFileOperationsRequest, batch ach.Batcher, filename string) error {
	header := batch.GetHeader()
	switch header.StandardEntryClassCode {
	case ach.CCD: // Corporate Credit or Debit Entry
		return nil

	case ach.PPD: // Prearranged Payment and Deposit Entry
		return nil

	case ach.TEL: // Telephone Initiated Entry
		return nil

	case ach.WEB: // Internet-Initiated/Mobile Entry
		return nil

	case ach.COR: // Notification of Change or Refused Notification of Change
		return errors.New("COR/NOC shouldn't be here, it should have been picked up by processInboundFiles")

	// SEC codes we'll reject because they're not implemented
	case
		ach.ACK, // ACH Payment Acknowledgment
		ach.ADV, // Automated Accounting Advice
		ach.ARC, // Accounts Receivable Entry (consumer check as a one-time ACH debit)
		ach.ATX, // Financial EDI Acknowledgment of CTX
		ach.BOC, // Back Office Conversion Entry
		ach.CIE, // Customer Initiated Entry
		ach.CTX, // Corporate Trade Exchange
		ach.DNE, // Death Notification Entry
		ach.ENR, // Automated Enrollment Entry
		ach.IAT, // International ACH Transaction
		ach.MTE, // Machine Transfer Entry
		ach.POP, // Point of Purchase Entry
		ach.POS, // Point of Sale Entry
		ach.RCK, // Re-presented Check Entry
		ach.SHR, // Shared Network Transaction
		ach.TRC, // Check Truncation Entry
		ach.TRX, // Check Truncation Entries Exchange
		ach.XCK: // Destroyed Check Entry
		return fmt.Errorf("unimplemented SEC code: %s", header.StandardEntryClassCode)

	default:
		return fmt.Errorf("unandled SEC code: %s", header.StandardEntryClassCode)
	}
	return nil
}
func (c *Controller) findOriginator(userID id.User, bh *ach.BatchHeader) (*model.Originator, error) {
	originators, err := c.origRepo.GetUserOriginators(userID)
	if err != nil {
		return nil, fmt.Errorf("unable to query originators: %v", err)
	}
	fmt.Printf("originators=%#v\n", originators)

	return nil, errors.New("TODO")
}

func createTransferFromEntry(userID id.User, fh ach.FileHeader, bh *ach.BatchHeader, entry *ach.EntryDetail) (*model.Transfer, error) {
	xfer := &model.Transfer{
		ID:                     id.Transfer(base.ID()),
		Description:            entry.DiscretionaryData,
		SameDay:                strings.HasPrefix(bh.CompanyDescriptiveDate, "SD"),
		StandardEntryClassCode: bh.StandardEntryClassCode,
		Status:                 model.TransferProcessed,
		UserID:                 userID.String(),
		Created:                base.NewTime(time.Now()),
		// Originator OriginatorID `json:"originator"`
		// OriginatorDepository id.Depository `json:"originatorDepository"`
		// Receiver ReceiverID `json:"receiver"`
		// ReceiverDepository id.Depository `json:"receiverDepository"`
	}
	if amt, err := model.NewAmountFromInt("USD", entry.Amount); amt != nil && err == nil {
		xfer.Amount = *amt
	} else {
		return nil, fmt.Errorf("bad amount: %v", err)
	}

	// Set transfer type from transaction code
	switch entry.TransactionCode {
	case ach.CheckingCredit, ach.SavingsCredit, ach.GLCredit, ach.LoanCredit:
		xfer.Type = model.PushTransfer

	case ach.CheckingDebit, ach.SavingsDebit, ach.GLDebit, ach.LoanDebit:
		xfer.Type = model.PullTransfer

	default:
		return nil, fmt.Errorf("unhandled TransactionCode=%d with TraceNumber=%s", entry.TransactionCode, entry.TraceNumber)
	}

	return xfer, nil
}

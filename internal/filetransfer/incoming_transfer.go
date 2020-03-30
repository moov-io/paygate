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

	cfg := c.findFileTransferConfig(file.Header.ImmediateOrigin)
	if cfg == nil {
		return fmt.Errorf("unable to find Config for %s", file.Header.ImmediateOrigin)
	}

	for i := range file.Batches {
		// For some SEC codes reject them right away as we don't currently support them. Returned these files to the ODFI.
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
			dep, err := c.rejectEntryFromDepositoryStatus(req, cfg, file.Header, file.Batches[i], entries[j])
			if err != nil {
				c.logger.Log(
					"handleIncomingTransfer", fmt.Sprintf("unable to process incoming EntryDetail traceNumber=%s: %v", entries[j].TraceNumber, err),
					"userID", req.userID, "requestID", req.requestID)
				continue
			}
			if dep == nil || dep.ID == "" {
				c.logger.Log(
					"handleIncomingTransfer", fmt.Sprintf("depository not found traceNumber=%s", entries[j].TraceNumber),
					"userID", req.userID, "requestID", req.requestID)

				// R03 may not be used to return ARC, BOC or POP entries solely because they do not contain an Individual Name.
				//
				// {"R03", "No Account/Unable to Locate Account",
				//   "Account number structure is valid and passes editing process, but does not correspond to individual or is not an open account"}

				continue
			}

			xfer, err := createTransferFromEntry(dep.UserID, file.Header, file.Batches[i].GetHeader(), entries[j])
			if err != nil {
				fmt.Printf("error=%v\n", err)
			}

			fmt.Printf("\nxfer=%#v\n\n", xfer)

			if c.accountsClient != nil {
				// Find account
				// c.accountsClient.SearchAccounts(requestID string, userID id.User, dep *model.Depository) (*Account, error)
				//
				// Post Transaction
				// PostTransaction(requestID string, userID id.User, lines []TransactionLine) (*Transaction, error)
				//
				// xfer.TransactionID string `json:"-"`
				//
				// Needs to support "settlementDate": "2020-01-01", on Transaction

				// {"R01", "Insufficient Funds", "Available balance is not sufficient to cover the dollar value of the debit entry"},
			}
		}
	}

	return nil
}

func (c *Controller) rejectEntryFromDepositoryStatus(req *periodicFileOperationsRequest, cfg *Config, fh ach.FileHeader, batch ach.Batcher, entry *ach.EntryDetail) (*model.Depository, error) {
	// Section 3.1.2 allows an RDFI to rely on account number in the EntryDetail to post transactions.
	//
	// TODO(adam): This might not work well for us as multiple users could have the same routing/account number pairs.
	// Also, we should likely limit these to Originators and exclude Receivers.
	dep, err := c.depRepo.LookupDepository(fh.ImmediateDestination, entry.DFIAccountNumber)
	if err != nil {
		c.logger.Log(
			"handleIncomingTransfer", fmt.Sprintf("unable to find depository: %v", err),
			"userID", req.userID, "requestID", req.requestID)
		return nil, err
	}

	var returnCode string
	if dep.Status == model.DepositoryDeceased {
		returnCode = "R14" // Representative payee deceased or unable to continue in that capacity
	}
	if dep.Status == model.DepositoryFrozen {
		returnCode = "R16" // Bank account frozen
	}
	if returnCode != "" {
		out, err := returnEntry(fh, batch, entry, returnCode)
		if err != nil {
			return nil, fmt.Errorf("problem creating return for EntryDetail traceNumber=%s: %v", entry.TraceNumber, err)
		}
		if err := c.writeReturnFile(cfg, out); err != nil {
			return nil, fmt.Errorf("problem writing return for EntryDetail traceNumber=%s: %v", entry.TraceNumber, err)
		}
	}
	return nil, nil
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

		// {"R30", "RDFI not participant in check truncation program", "Financial institution not participating in automated check safekeeping application"},

	default:
		return fmt.Errorf("unandled SEC code: %s", header.StandardEntryClassCode)
	}
	return nil
}

// We must initiate a correction file and return file at the same time (if the error is correctable)

// Perform an OFAC search?
// Need to handle DNE's and mark objects as such

// Create "Fee Entry" files where we can charge RDFI for return file

// Incoming credits have a settlement date which can be in the future we need to account for

// handle prenote files, we should send them as a verification option too

// {"R02", "Account Closed", "Previously active account has been closed by customer or RDFI"},
// {"R04", "Invalid Account Number", "Account number structure not valid; entry may fail check digit validation or may contain an incorrect number of digits."},

// {"R05", "Improper Debit to Consumer Account", "A CCD, CTX, or CBR debit entry was transmitted to a Consumer Account of the Receiver and was not authorized by the Receiver"},
// {"R06", "Returned per ODFI's Request", "ODFI has requested RDFI to return the ACH entry (optional to RDFI - ODFI indemnifies RDFI)}"},

// R07 Prohibited use for ARC, BOC, POP and RCK.
// {"R07", "Authorization Revoked by Customer", "Consumer, who previously authorized ACH payment, has revoked authorization from Originator (must be returned no later than 60 days from settlement date and customer must sign affidavit)"},

// {"R08", "Payment Stopped", "Receiver of a recurring debit transaction has stopped payment to a specific ACH debit. RDFI should verify the Receiver's intent when a request for stop payment is made to insure this is not intended to be a revocation of authorization"},
// {"R10", "Customer Advises Not Authorized", "Consumer has advised RDFI that Originator of transaction is not authorized to debit account (must be returned no later than 60 days from settlement date of original entry and customer must sign affidavit)."},

// {"R12", "Branch Sold to Another DFI", "Financial institution receives entry destined for an account at a branch that has been sold to another financial institution."},
// {"R13", "RDFI not qualified to participate", "Financial institution does not receive commercial ACH entries"},

// {"R14", "Representative payee deceased or unable to continue in that capacity", "The representative payee authorized to accept entries on behalf of a beneficiary is either deceased or unable to continue in that capacity"},
// {"R15", "Beneficiary or bank account holder", "(Other than representative payee) deceased* - (1) the beneficiary entitled to payments is deceased or (2) the bank account holder other than a representative payee is deceased"},

// {"R16", "Bank account frozen", "Funds in bank account are unavailable due to action by RDFI or legal order"},

// {"R17", "File record edit criteria", "Fields rejected by RDFI processing (identified in return addenda)"},
// {"R18", "Improper effective entry date", "Entries have been presented prior to the first available processing window for the effective date."},

// would be returned in non-zero amount for a prenotification
// {"R19", "Amount field error", "Improper formatting of the amount field"},

// {"R20", "Non-payment bank account", "Entry destined for non-payment bank account defined by reg."},

// {"R21", "Invalid company ID number", "The company ID information not valid (normally CIE entries)"},
// {"R22", "Invalid individual ID number", "Individual id used by receiver is incorrect (CIE entries)"},
// {"R28", "Transit routing number check digit error", "Check digit for the transit routing number is incorrect"},

// {"R23", "Credit entry refused by receiver", "Receiver returned entry because minimum or exact amount not remitted, bank account is subject to litigation, or payment represents an overpayment, originator is not known to receiver or receiver has not authorized this credit entry to this bank account"},
// {"R29", "Corporate customer advises not authorized", "RDFI has bee notified by corporate receiver that debit entry of originator is not authorized"},

// {"R24", "Duplicate entry", "RDFI has received a duplicate entry"},

// {"R30", "RDFI not participant in check truncation program", "Financial institution not participating in automated check safekeeping application"},

// {"R31", "Permissible return entry (CCD and CTX only)", "RDFI has been notified by the ODFI that it agrees to accept a CCD or CTX return entry"},

// {"R32", "RDFI non-settlement", "RDFI is not able to settle the entry"},

// {"R33", "Return of XCK entry", "RDFI determines at its sole discretion to return an XCK entry; an XCK return entry may be initiated by midnight of the sixtieth day following the settlement date if the XCK entry"},

// {"R34", "Limited participation RDFI", "RDFI participation has been limited by a federal or state supervisor"},

// {"R35", "Return of improper debit entry", "ACH debit not permitted for use with the CIE standard entry class code (except for reversals)"},

// {"R40", "Return of ENR Entry by Federal Government Agency (ENR Only)", "This return reason code may only be used to return ENR entries and is at the federal Government Agency's Sole discretion"},
// {"R41", "Invalid Transaction Code (ENR only)", "Either the Transaction Code included in Field 3 of the Addenda Record does not conform to the ACH Record Format Specifications contained in Appendix Three (ACH Record Format Specifications) or it is not appropriate with regard to an Automated Enrollment Entry."},
// {"R42", "Routing Number/Check Digit Error (ENR Only)", "The Routing Number and the Check Digit included in Field 3 of the Addenda Record is either not a valid number or it does not conform to the Modulus 10 formula."},
// {"R43", "Invalid DFI Account Number (ENR Only)", "The Receiver's account number included in Field 3 of the Addenda Record must include at least one alphameric character."},
// {"R44", "Invalid Individual ID Number/Identification Number (ENR only)", "The Individual ID Number/Identification Number provided in Field 3 of the Addenda Record does not match a corresponding ID number in the Federal Government Agency's records."},
// {"R45", "Invalid Individual Name/Company Name (ENR only)", "The name of the consumer or company provided in Field 3 of the Addenda Record either does not match a corresponding name in the Federal Government Agency's records or fails to include at least one alphameric character."},
// {"R46", "Invalid Representative Payee Indicator (ENR Only)", "The Representative Payee Indicator Code included in Field 3 of the Addenda Record has been omitted or it is not consistent with the Federal Government Agency's records."},
// {"R47", "Duplicate Enrollment (ENR Only)", "The Entry is a duplicate of an Automated Enrollment Entry previously initiated by a DFI."},
//
// Return Codes to be used for RCK entries only and are initiated by a RDFI
// {"R50", "State Law Affecting RCK Acceptance", "RDFI is located in a state that has not adopted Revised Article 4 of the UCC or the RDFI is located in a state that requires all canceled checks to be returned within the periodic statement"},

// {"R51", "Item Related to RCK Entry is Ineligible or RCK Entry is Improper", "The item to which the RCK entry relates was not eligible, Originator did not provide notice of the RCK policy, signature on the item was not genuine, the item has been altered or amount of the entry was not accurately obtained from the item. RDFI must obtain a Written Statement and return the entry within 60 days following Settlement Date"},
// {"R52", "Stop Payment on Item (Adjustment Entry)", "A stop payment has been placed on the item to which the RCK entry relates. RDFI must return no later than 60 days following Settlement Date. No Written Statement is required as the original stop payment form covers the return."},
// {"R53", "Item and RCK Entry Presented for Payment (Adjustment Entry)", "Both the RCK entry and check have been presented forpayment. RDFI must obtain a Written Statement and return the entry within 60 days following Settlement Date"},

// Return Codes to be used by the ODFI for dishonored return entries
// {"R61", "Misrouted Return", "The financial institution preparing the Return Entry (the RDFI of the original Entry) has placed the incorrect Routing Number in the Receiving DFI Identification field."},
// {"R67", "Duplicate Return", "The ODFI has received more than one Return for the same Entry."},
// {"R68", "Untimely Return", "The Return Entry has not been sent within the time frame established by these Rules."},
// {"R69", "Field Error(s)", "One or more of the field requirements are incorrect."},
// {"R70", "Permissible Return Entry Not Accepted/Return Not Requested by ODFI", "The ODFI has received a Return Entry identified by the RDFI as being returned with the permission of, or at the request of, the ODFI, but the ODFI has not agreed to accept the Entry or has not requested the return of the Entry."},
// Return Codes to be used by the RDFI for contested dishonored return entries
// {"R71", "Misrouted Dishonored Return", "The financial institution preparing the dishonored Return Entry (the ODFI of the original Entry) has placed the incorrect Routing Number in the Receiving DFI Identification field."},
// {"R72", "Untimely Dishonored Return", "The dishonored Return Entry has not been sent within the designated time frame."},
// {"R73", "Timely Original Return", "The RDFI is certifying that the original Return Entry was sent within the time frame designated in these Rules."},
// {"R74", "Corrected Return", "The RDFI is correcting a previous Return Entry that was dishonored using Return Reason Code R69 (Field Error(s)) because it contained incomplete or incorrect information."},
// {"R75", "Return Not a Duplicate", "The Return Entry was not a duplicate of an Entry previously returned by the RDFI."},
// {"R76", "No Errors Found", "The original Return Entry did not contain the errors indicated by the ODFI in the dishonored Return Entry."},
//
// Return Codes to be used by Gateways for the return of international payments
// {"R80", "IAT Entry Coding Error", "The IAT Entry is being returned due to one or more of the following conditions: Invalid DFI/Bank Branch Country Code, invalid DFI/Bank Identification Number Qualifier, invalid Foreign Exchange Indicator, invalid ISO Originating Currency Code, invalid ISO Destination Currency Code, invalid ISO Destination Country Code, invalid Transaction Type Code"},
// {"R81", "Non-Participant in IAT Program", "The IAT Entry is being returned because the Gateway does not have an agreement with either the ODFI or the Gateway's customer to transmit Outbound IAT Entries."},
// {"R82", "Invalid Foreign Receiving DFI Identification", "The reference used to identify the Foreign Receiving DFI of an Outbound IAT Entry is invalid."},
// {"R83", "Foreign Receiving DFI Unable to Settle", "The IAT Entry is being returned due to settlement problems in the foreign payment system."},
// {"R84", "Entry Not Processed by Gateway", "For Outbound IAT Entries, the Entry has not been processed and is being returned at the Gateway's discretion because either (1) the processing of such Entry may expose the Gateway to excessive risk, or (2) the foreign payment system does not support the functions needed to process the transaction."},
// {"R85", "Incorrectly Coded Outbound International Payment", "The RDFI/Gateway has identified the Entry as an Outbound international payment and is returning the Entry because it bears an SEC Code that lacks information required by the Gateway for OFAC compliance."},

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
	if amt, err := model.NewAmountFromInt("USD", entry.Amount); entry.Amount > 0 && amt != nil && err == nil {
		xfer.Amount = *amt
	} else {
		// {"R19", "Amount field error", "Improper formatting of the amount field"},

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

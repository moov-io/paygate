// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/pkg/achclient"
)

var (
	// TODO(Adam): Once we have APIs for an account we'll need to have apitest read the transaction
	// for each micro-deposit and use those values.
	zzone, _                 = NewAmount("USD", "0.01")
	zzthree, _               = NewAmount("USD", "0.03")
	fixedMicroDepositAmounts = []Amount{*zzone, *zzthree}

	odfiRoutingNumber = or(os.Getenv("ODFI_ROUTING_NUMBER"), "121042882") // TODO(adam): something for local dev

	odfiOriginator = &Originator{
		ID:                "odfi", // TODO(adam): make this NOT querable via db.
		DefaultDepository: DepositoryID("odfi"),
		Identification:    or(os.Getenv("ODFI_IDENTIFICATION"), "001"), // TODO(Adam)
		Metadata:          "Moov - paygate micro-deposits",
	}
	odfiDepository = &Depository{
		ID:            DepositoryID("odfi"),
		BankName:      or(os.Getenv("ODFI_BANK_NAME"), "Moov, Inc"),
		Holder:        or(os.Getenv("ODFI_HOLDER"), "Moov, Inc"),
		HolderType:    Individual,
		Type:          Savings,
		RoutingNumber: odfiRoutingNumber,
		AccountNumber: or(os.Getenv("ODFI_ACCOUNT_NUMBER"), "123"),
		Status:        DepositoryVerified,
	}
)

// or returns primary if non-empty and backup otherwise
func or(primary, backup string) string {
	primary = strings.TrimSpace(primary)
	if primary == "" {
		return strings.TrimSpace(backup)
	}
	return primary
}

type microDeposit struct {
	amount Amount
	fileId string
}

// initiateMicroDeposits will write micro deposits into the underlying database and kick off the ACH transfer(s).
//
// Note: No money is actually transferred yet. Only fixedMicroDepositAmounts amounts are written
func initiateMicroDeposits(depRepo depositoryRepository, eventRepo eventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "initiateMicroDeposits")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), moovhttp.GetUserId(r)
		if id == "" {
			// 404 - A depository with the specified ID was not found.
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Check the depository status and confirm it belongs to the user
		dep, err := depRepo.getUserDepository(id, userId)
		if err != nil {
			logger.Log("microDeposits", err)
			internalError(w, err)
			return
		}
		if dep.Status != DepositoryUnverified {
			err = fmt.Errorf("(userId=%s) depository %s in bogus status %s", userId, dep.ID, dep.Status)
			logger.Log("microDeposits", err)
			moovhttp.Problem(w, err)
			return
		}

		// Our Depository needs to be Verified so let's submit some micro deposits to it.
		microDeposits, err := submitMicroDeposits(userId, fixedMicroDepositAmounts, dep, depRepo, eventRepo)
		if err != nil {
			err = fmt.Errorf("(userId=%s) had problem submitting micro-deposits: %v", userId, err)
			if logger != nil {
				logger.Log("microDeposits", err)
			}
			moovhttp.Problem(w, err)
			return
		}

		// Write micro deposits into our db
		if err := depRepo.initiateMicroDeposits(id, userId, microDeposits); err != nil {
			if logger != nil {
				logger.Log("microDeposits", err)
			}
			internalError(w, err)
			return
		}

		// 201 - Micro deposits initiated
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("{}"))
	}
}

// submitMicroDeposits will create ACH files to process multiple micro-deposit transfers to validate a Depository.
// The Originator used belongs to the ODFI (or Moov in tests).
//
// The steps needed are:
// - Grab related transfer objects for the user
// - Create several Transfers and create their ACH files (then validate)
// - Write micro-deposits to SQL table (used in /confirm endpoint)
//
//
// TODO(adam): misc things
// TODO(adam): reject if user has been failed too much verifying this Depository -- w.WriteHeader(http.StatusConflict)
func submitMicroDeposits(userId string, amounts []Amount, dep *Depository, depRepo depositoryRepository, eventRepo eventRepository) ([]microDeposit, error) {
	var microDeposits []microDeposit
	for i := range amounts {
		req := &transferRequest{
			Type:                   PushTransfer,
			Amount:                 amounts[i],
			Originator:             odfiOriginator.ID, // e.g. Moov, Inc
			OriginatorDepository:   odfiDepository.ID,
			Description:            fmt.Sprintf("%s micro-deposit verification", odfiDepository.BankName),
			StandardEntryClassCode: "PPD",
		}

		// The Receiver and ReceiverDepository are the Depository that needs approval.
		req.Receiver = ReceiverID(fmt.Sprintf("%s-micro-deposit-verify-%s", userId, nextID()[:8]))
		req.ReceiverDepository = dep.ID
		cust := &Receiver{
			ID:       req.Receiver,
			Status:   ReceiverVerified, // Something to pass createACHFile validation logic
			Metadata: dep.Holder,       // Depository holder is getting the micro deposit
		}

		// Convert to Transfer object
		xfer := req.asTransfer(string(cust.ID))

		// Submit the file to our ACH service
		ach := achclient.New(userId, logger)
		fileId, err := createACHFile(ach, string(xfer.ID), nextID(), userId, xfer, cust, dep, odfiOriginator, odfiDepository)
		if err != nil {
			err = fmt.Errorf("problem creating ACH file for userId=%s: %v", userId, err)
			if logger != nil {
				logger.Log("microDeposits", err)
			}
			return nil, err
		}

		if err := checkACHFile(ach, fileId, userId); err != nil {
			return nil, err
		}

		// TODO(adam): We shouldn't be deleting these files. They'll need to be merged and shipped off to the Fed.
		// However, for now we're deleting them to keep the ACH (and moov.io/demo) cleaned up of ACH files.
		if err := ach.DeleteFile(fileId); err != nil {
			return nil, fmt.Errorf("ach DeleteFile: %v", err)
		}

		if err := writeTransferEvent(userId, req, eventRepo); err != nil {
			return nil, fmt.Errorf("userId=%s problem writing micro-deposit transfer event: %v", userId, err)
		}

		microDeposits = append(microDeposits, microDeposit{
			amount: amounts[i],
			fileId: fileId,
		})
	}
	return microDeposits, nil
}

type confirmDepositoryRequest struct {
	Amounts []string `json:"amounts"`
}

// confirmMicroDeposits checks our database for a depository's micro deposits (used to validate the user owns the Depository)
// and if successful changes the Depository status to DepositoryVerified.
func confirmMicroDeposits(repo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "confirmMicroDeposits")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), moovhttp.GetUserId(r)
		if id == "" {
			// 404 - A depository with the specified ID was not found.
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Check the depository status and confirm it belongs to the user
		dep, err := repo.getUserDepository(id, userId)
		if err != nil {
			internalError(w, err)
			return
		}
		if dep.Status != DepositoryUnverified {
			moovhttp.Problem(w, fmt.Errorf("(userId=%s) depository %s in bogus status %s", userId, dep.ID, dep.Status))
			return
		}

		// TODO(adam): check depository status
		// 409 - Too many attempts. Bank already verified. // w.WriteHeader(http.StatusConflict)

		// Read amounts from request JSON
		var req confirmDepositoryRequest
		rr := io.LimitReader(r.Body, maxReadBytes)
		if err := json.NewDecoder(rr).Decode(&req); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		var amounts []Amount
		for i := range req.Amounts {
			amt := &Amount{}
			if err := amt.FromString(req.Amounts[i]); err != nil {
				continue
			}
			amounts = append(amounts, *amt)
		}
		if len(amounts) == 0 {
			// 400 - Invalid Amounts
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := repo.confirmMicroDeposits(id, userId, amounts); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		// Update Depository status
		if err := markDepositoryVerified(repo, id, userId); err != nil {
			internalError(w, err)
			return
		}

		// 200 - Micro deposits verified
		w.WriteHeader(http.StatusOK)
	}
}

// getMicroDeposits will retrieve the micro deposits for a given depository. If an amount does not parse it will be discardded silently.
func (r *sqliteDepositoryRepo) getMicroDeposits(id DepositoryID, userId string) ([]microDeposit, error) {
	query := `select amount, file_id from micro_deposits where user_id = ? and depository_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userId, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var microDeposits []microDeposit
	for rows.Next() {
		var fileId string
		var value string
		if err := rows.Scan(&value, &fileId); err != nil {
			continue
		}

		amt := &Amount{}
		if err := amt.FromString(value); err != nil {
			continue
		}
		microDeposits = append(microDeposits, microDeposit{
			amount: *amt,
			fileId: fileId,
		})
	}

	return microDeposits, nil
}

// initiateMicroDeposits will save the provided []Amount into our database. If amounts have already been saved then
// no new amounts will be added.
func (r *sqliteDepositoryRepo) initiateMicroDeposits(id DepositoryID, userId string, microDeposits []microDeposit) error {
	existing, err := r.getMicroDeposits(id, userId)
	if err != nil || len(existing) > 0 {
		return fmt.Errorf("not initializing more micro deposits, already have %d or got error=%v", len(existing), err)
	}

	// write amounts
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	now, query := time.Now(), `insert into micro_deposits (depository_id, user_id, amount, file_id, created_at) values (?, ?, ?, ?, ?)`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range microDeposits {
		_, err = stmt.Exec(id, userId, microDeposits[i].amount.String(), microDeposits[i].fileId, now)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// confirmMicroDeposits will compare the provided guessAmounts against what's been persisted for a user. If the amounts do not match
// or there are a mismatched amount the call will return a non-nil error.
func (r *sqliteDepositoryRepo) confirmMicroDeposits(id DepositoryID, userId string, guessAmounts []Amount) error {
	microDeposits, err := r.getMicroDeposits(id, userId)
	if err != nil || len(microDeposits) == 0 {
		return fmt.Errorf("unable to confirm micro deposits, got %d micro deposits or error=%v", len(microDeposits), err)
	}

	// Check amounts, all must match
	if (len(guessAmounts) < len(microDeposits)) || len(guessAmounts) == 0 {
		return fmt.Errorf("incorrect amount of guesses, got %d", len(guessAmounts)) // don't share len(microDeposits), that's an info leak
	}

	found := 0
	for i := range microDeposits {
		for k := range guessAmounts {
			if microDeposits[i].amount.Equal(guessAmounts[k]) {
				found += 1
			}
		}
	}

	if found != len(microDeposits) {
		return errors.New("incorrect micro deposit guesses")
	}

	return nil
}

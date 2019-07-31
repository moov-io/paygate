// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	accounts "github.com/moov-io/accounts/client"
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
)

// odfiAccount represents the depository account micro-deposts are debited from
type odfiAccount struct {
	accountNumber string
	routingNumber string
	accountType   AccountType

	client AccountsClient

	mu        sync.Mutex
	accountId string
}

func (a *odfiAccount) getID(requestId, userId string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.accountId != "" {
		return a.accountId, nil
	}
	if a.client == nil {
		return "", errors.New("odfiAccount: nil AccountsClient")
	}

	// Otherwise, make our Accounts HTTP call and grab the ID
	dep := &Depository{
		AccountNumber: a.accountNumber,
		RoutingNumber: a.routingNumber,
		Type:          a.accountType,
	}
	acct, err := a.client.SearchAccounts(requestId, userId, dep)
	if err != nil || (acct == nil || acct.Id == "") {
		return "", fmt.Errorf("odfiAccount: problem getting accountId: %v", err)
	}
	a.accountId = acct.Id // record account ID for calls later on
	return a.accountId, nil
}

func (a *odfiAccount) metadata() (*Originator, *Depository) {
	orig := &Originator{
		ID:                "odfi", // TODO(adam): make this NOT querable via db.
		DefaultDepository: DepositoryID("odfi"),
		Identification:    or(os.Getenv("ODFI_IDENTIFICATION"), "001"),
		Metadata:          "Moov - paygate micro-deposits",
	}
	dep := &Depository{
		ID:            DepositoryID("odfi"),
		BankName:      or(os.Getenv("ODFI_BANK_NAME"), "Moov, Inc"),
		Holder:        or(os.Getenv("ODFI_HOLDER"), "Moov, Inc"),
		HolderType:    Individual,
		Type:          a.accountType,
		RoutingNumber: a.routingNumber,
		AccountNumber: a.accountNumber,
		Status:        DepositoryVerified,
	}
	return orig, dep
}

type microDeposit struct {
	amount Amount
	fileId string
}

func microDepositAmounts() []Amount {
	rand := func() int {
		n, _ := rand.Int(rand.Reader, big.NewInt(49)) // rand.Int returns [0, N) and we want a range of $0.01 to $0.50
		return int(n.Int64()) + 1
	}
	// generate two amounts and a third that's the sum
	n1, n2 := rand(), rand()
	a1, _ := NewAmount("USD", fmt.Sprintf("0.%02d", n1)) // pad 1 to '01'
	a2, _ := NewAmount("USD", fmt.Sprintf("0.%02d", n2))
	a3, _ := NewAmount("USD", fmt.Sprintf("0.%02d", n1+n2))
	return []Amount{*a1, *a2, *a3}
}

// initiateMicroDeposits will write micro deposits into the underlying database and kick off the ACH transfer(s).
//
func initiateMicroDeposits(logger log.Logger, odfiAccount *odfiAccount, accountsClient AccountsClient, achClient *achclient.ACH, depRepo depositoryRepository, eventRepo eventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
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
			internalError(logger, w, err)
			return
		}
		if dep.Status != DepositoryUnverified {
			err = fmt.Errorf("(userId=%s) depository %s in bogus status %s", userId, dep.ID, dep.Status)
			logger.Log("microDeposits", err)
			moovhttp.Problem(w, err)
			return
		}

		// Our Depository needs to be Verified so let's submit some micro deposits to it.
		amounts, requestId := microDepositAmounts(), moovhttp.GetRequestId(r)
		microDeposits, err := submitMicroDeposits(logger, odfiAccount, accountsClient, userId, requestId, amounts, dep, depRepo, eventRepo, achClient)
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
			internalError(logger, w, err)
			return
		}

		// 201 - Micro deposits initiated
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("{}"))
	}
}

func postMicroDepositTransaction(logger log.Logger, client AccountsClient, accountId, userId string, lines []transactionLine, requestId string) (*accounts.Transaction, error) {
	var transaction *accounts.Transaction
	var err error
	for i := 0; i < 3; i++ {
		transaction, err = client.PostTransaction(requestId, userId, lines)
		if err == nil {
			break // quit after successful call
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error creating transaction for transfer user=%s: %v", userId, err)
	}
	logger.Log("transfers", fmt.Sprintf("created transaction=%s for user=%s", transaction.Id, userId), "requestId", requestId)
	return transaction, nil
}

func postMicroDepositTransactions(logger log.Logger, odfiAccount *odfiAccount, client AccountsClient, userId string, dep *Depository, amounts []Amount, requestId string) ([]*accounts.Transaction, error) {
	if len(amounts) != 3 {
		return nil, fmt.Errorf("postMicroDepositTransactions: unexpected %d Amounts", len(amounts))
	}
	acct, err := client.SearchAccounts(requestId, userId, dep)
	if err != nil || acct == nil {
		return nil, fmt.Errorf("error reading account user=%s depository=%s: %v", userId, dep.ID, err)
	}
	odfiAccountId, err := odfiAccount.getID(requestId, userId)
	if err != nil {
		return nil, fmt.Errorf("posting micro-deposits: %v", err)
	}

	// Submit all micro-deposits
	var transactions []*accounts.Transaction
	for i := range amounts {
		lines := []transactionLine{
			{AccountId: acct.Id, Purpose: "ACHCredit", Amount: int32(amounts[i].Int())},
			{AccountId: odfiAccountId, Purpose: "ACHDebit", Amount: int32(amounts[i].Int())},
		}
		if i == 2 { // our last Amount undos the credits to the external account
			lines[0].Purpose = "ACHDebit"
			lines[1].Purpose = "ACHCredit"
		}
		tx, err := postMicroDepositTransaction(logger, client, acct.Id, userId, lines, requestId)
		if err != nil {
			return nil, err // we retried and failed, so just exit early
		}
		transactions = append(transactions, tx)
	}
	return transactions, nil
}

// submitMicroDeposits will create ACH files to process multiple micro-deposit transfers to validate a Depository.
// The Originator used belongs to the ODFI (or Moov in tests).
//
// The steps needed are:
// - Grab related transfer objects for the user
// - Create several Transfers and create their ACH files (then validate)
// - Write micro-deposits to SQL table (used in /confirm endpoint)
//
// submitMicroDeposits assumes there are 2 amounts to credit and a third to debit.
//
// TODO(adam): reject if user has been failed too much verifying this Depository -- w.WriteHeader(http.StatusConflict)
func submitMicroDeposits(logger log.Logger, odfiAccount *odfiAccount, client AccountsClient, userId string, requestId string, amounts []Amount, dep *Depository, depRepo depositoryRepository, eventRepo eventRepository, achClient *achclient.ACH) ([]microDeposit, error) {
	odfiOriginator, odfiDepository := odfiAccount.metadata()

	var microDeposits []microDeposit
	for i := range amounts {
		req := &transferRequest{
			Amount:                 amounts[i],
			Originator:             odfiOriginator.ID, // e.g. Moov, Inc
			OriginatorDepository:   odfiDepository.ID,
			Description:            fmt.Sprintf("%s micro-deposit verification", odfiDepository.BankName),
			StandardEntryClassCode: ach.PPD,
		}
		// micro-deposits must balance, the 3rd amount is the other two's sum
		if i == 0 || i == 1 {
			req.Type = PushTransfer
		} else {
			req.Type = PullTransfer
		}

		// The Receiver and ReceiverDepository are the Depository that needs approval.
		req.Receiver = ReceiverID(fmt.Sprintf("%s-micro-deposit-verify-%s", userId, base.ID()[:8]))
		req.ReceiverDepository = dep.ID
		cust := &Receiver{
			ID:       req.Receiver,
			Status:   ReceiverVerified, // Something to pass createACHFile validation logic
			Metadata: dep.Holder,       // Depository holder is getting the micro deposit
		}

		// Convert to Transfer object
		xfer := req.asTransfer(string(cust.ID))

		// Submit the file to our ACH service
		fileId, err := createACHFile(achClient, string(xfer.ID), base.ID(), userId, xfer, cust, dep, odfiOriginator, odfiDepository)
		if err != nil {
			err = fmt.Errorf("problem creating ACH file for userId=%s: %v", userId, err)
			if logger != nil {
				logger.Log("microDeposits", err)
			}
			return nil, err
		}
		if err := checkACHFile(logger, achClient, fileId, userId); err != nil {
			return nil, err
		}

		// TODO(adam): We need to add these transactions into ACH files uploaded to our SFTP credentials

		// TODO(adam): We shouldn't be deleting these files. They'll need to be merged and shipped off to the Fed.
		// However, for now we're deleting them to keep the ACH (and moov.io/demo) cleaned up of ACH files.
		if err := achClient.DeleteFile(fileId); err != nil {
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
	// Post the transaction against Accounts only if it's enabled (flagged via nil AccountsClient)
	if client != nil {
		transactions, err := postMicroDepositTransactions(logger, odfiAccount, client, userId, dep, amounts, requestId)
		if err != nil {
			return microDeposits, fmt.Errorf("submitMicroDeposits: error posting to Accounts: %v", err)
		}
		logger.Log("microDeposits", fmt.Sprintf("created %d transactions for user=%s micro-deposits", len(transactions), userId), "requestId", requestId)
	}
	return microDeposits, nil
}

type confirmDepositoryRequest struct {
	Amounts []string `json:"amounts"`
}

// confirmMicroDeposits checks our database for a depository's micro deposits (used to validate the user owns the Depository)
// and if successful changes the Depository status to DepositoryVerified.
func confirmMicroDeposits(logger log.Logger, repo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
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
			internalError(logger, w, err)
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
			moovhttp.Problem(w, errors.New("invalid amounts, found none"))
			return
		}
		if err := repo.confirmMicroDeposits(id, userId, amounts); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		// Update Depository status
		if err := markDepositoryVerified(repo, id, userId); err != nil {
			internalError(logger, w, err)
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
	return microDeposits, rows.Err()
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
		return fmt.Errorf("initiateMicroDeposits: prepare error=%v rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()

	for i := range microDeposits {
		_, err = stmt.Exec(id, userId, microDeposits[i].amount.String(), microDeposits[i].fileId, now)
		if err != nil {
			return fmt.Errorf("initiateMicroDeposits: scan error=%v rollback=%v", err, tx.Rollback())
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
				break
			}
		}
	}

	if found != len(microDeposits) && found > 0 {
		return errors.New("incorrect micro deposit guesses")
	}

	return nil
}

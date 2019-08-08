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

func microDepositAmounts() ([]Amount, int) {
	rand := func() int {
		n, _ := rand.Int(rand.Reader, big.NewInt(49)) // rand.Int returns [0, N) and we want a range of $0.01 to $0.50
		return int(n.Int64()) + 1
	}
	// generate two amounts and a third that's the sum
	n1, n2 := rand(), rand()
	a1, _ := NewAmount("USD", fmt.Sprintf("0.%02d", n1)) // pad 1 to '01'
	a2, _ := NewAmount("USD", fmt.Sprintf("0.%02d", n2))
	return []Amount{*a1, *a2}, n1 + n2
}

// initiateMicroDeposits will write micro deposits into the underlying database and kick off the ACH transfer(s).
//
func initiateMicroDeposits(logger log.Logger, odfiAccount *odfiAccount, accountsClient AccountsClient, achClient *achclient.ACH, depRepo depositoryRepository, eventRepo eventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		requestId := moovhttp.GetRequestId(r)

		id, userId := getDepositoryId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			// 404 - A depository with the specified ID was not found.
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "depository not found"}`))
			return
		}

		// Check the depository status and confirm it belongs to the user
		dep, err := depRepo.getUserDepository(id, userId)
		if err != nil {
			logger.Log("microDeposits", err, "requestId", requestId, "userId", userId)
			moovhttp.Problem(w, err)
			return
		}
		if dep.Status != DepositoryUnverified {
			err = fmt.Errorf("depository %s in bogus status %s", dep.ID, dep.Status)
			logger.Log("microDeposits", err, "requestId", requestId, "userId", userId)
			moovhttp.Problem(w, err)
			return
		}

		// Our Depository needs to be Verified so let's submit some micro deposits to it.
		amounts, sum := microDepositAmounts()
		microDeposits, err := submitMicroDeposits(logger, odfiAccount, accountsClient, userId, requestId, amounts, sum, dep, depRepo, eventRepo, achClient)
		if err != nil {
			err = fmt.Errorf("problem submitting micro-deposits: %v", err)
			if logger != nil {
				logger.Log("microDeposits", err, "requestId", requestId, "userId", userId)
			}
			moovhttp.Problem(w, err)
			return
		}
		logger.Log("microDeposits", fmt.Sprintf("submitted %d micro-deposits for depository=%s", len(microDeposits), dep.ID), "requestId", requestId, "userId", userId)

		// Write micro deposits into our db
		if err := depRepo.initiateMicroDeposits(id, userId, microDeposits); err != nil {
			if logger != nil {
				logger.Log("microDeposits", err, "requestId", requestId, "userId", userId)
			}
			moovhttp.Problem(w, err)
			return
		}
		logger.Log("microDeposits", fmt.Sprintf("stored micro-deposits for depository=%s", dep.ID), "requestId", requestId, "userId", userId)

		w.WriteHeader(http.StatusCreated) // 201 - Micro deposits initiated
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

func postMicroDepositTransactions(logger log.Logger, odfiAccount *odfiAccount, client AccountsClient, userId string, dep *Depository, amounts []Amount, sum int, requestId string) ([]*accounts.Transaction, error) {
	if len(amounts) != 2 {
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
		tx, err := postMicroDepositTransaction(logger, client, acct.Id, userId, lines, requestId)
		if err != nil {
			return nil, err // we retried and failed, so just exit early
		}
		transactions = append(transactions, tx)
	}
	// submit the reversal of our micro-deposits
	lines := []transactionLine{
		{AccountId: acct.Id, Purpose: "ACHDebit", Amount: int32(sum)},
		{AccountId: odfiAccountId, Purpose: "ACHCredit", Amount: int32(sum)},
	}
	tx, err := postMicroDepositTransaction(logger, client, acct.Id, userId, lines, requestId)
	if err != nil {
		return nil, fmt.Errorf("postMicroDepositTransaction: on sum transaction post: %v", err)
	}
	transactions = append(transactions, tx)
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
func submitMicroDeposits(logger log.Logger, odfiAccount *odfiAccount, client AccountsClient, userId string, requestId string, amounts []Amount, sum int, dep *Depository, depRepo depositoryRepository, eventRepo eventRepository, achClient *achclient.ACH) ([]microDeposit, error) {
	odfiOriginator, odfiDepository := odfiAccount.metadata()

	// TODO(adam): reject if user has been failed too much verifying this Depository -- w.WriteHeader(http.StatusConflict)

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
				logger.Log("microDeposits", err, "requestId", requestId)
			}
			return nil, err
		}
		if err := checkACHFile(logger, achClient, fileId, userId); err != nil {
			return nil, err
		}
		logger.Log("microDeposits", fmt.Sprintf("created ACH file=%s depository=%s", xfer.ID, dep.ID), "requestId", requestId)

		// TODO(adam): We need to add these transactions into ACH files uploaded to our SFTP credentials
		//
		// TODO(adam): We shouldn't be deleting these files. They'll need to be merged and shipped off to the Fed.
		// However, for now we're deleting them to keep the ACH (and moov.io/demo) cleaned up of ACH files.
		// if err := achClient.DeleteFile(fileId); err != nil {
		// 	return nil, fmt.Errorf("ach DeleteFile: %v", err)
		// }

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
		transactions, err := postMicroDepositTransactions(logger, odfiAccount, client, userId, dep, amounts, sum, requestId)
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
//
// TODO(adam): Should we allow a Depository to be confirmed before the micro-deposit ACH file is
// upload? Technically there's really no way for an end-user to see them before posting, however
// out demo and tests can lookup in Accounts right away and quickly verify the Depository.
func confirmMicroDeposits(logger log.Logger, repo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(logger, w, r)
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), moovhttp.GetUserId(r)
		if id == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			// 404 - A depository with the specified ID was not found.
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "depository not found"}`))
			return
		}

		// Check the depository status and confirm it belongs to the user
		dep, err := repo.getUserDepository(id, userId)
		if err != nil {
			logger.Log("confirmMicroDeposits", err, "userId", userId)
			moovhttp.Problem(w, err)
			return
		}
		if dep.Status != DepositoryUnverified {
			err = fmt.Errorf("depository %s in bogus status %s", dep.ID, dep.Status)
			logger.Log("confirmMicroDeposits", err, "userId", userId)
			moovhttp.Problem(w, err)
			return
		}

		// TODO(adam): if we've failed too many times return '409 - Too many attempts'

		// Read amounts from request JSON
		var req confirmDepositoryRequest
		rr := io.LimitReader(r.Body, maxReadBytes)
		if err := json.NewDecoder(rr).Decode(&req); err != nil {
			logger.Log("confirmDepositoryRequest", fmt.Sprintf("problem reading request: %v", err), "userId", userId)
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
			logger.Log("confirmMicroDeposits", "no micro-deposit amounts found", "userId", userId)
			// 400 - Invalid Amounts
			moovhttp.Problem(w, errors.New("invalid amounts, found none"))
			return
		}
		if err := repo.confirmMicroDeposits(id, userId, amounts); err != nil {
			logger.Log("confirmMicroDeposits", fmt.Sprintf("problem confirming micro-deposits: %v", err), "userId", userId)
			moovhttp.Problem(w, err)
			return
		}

		// Update Depository status
		if err := markDepositoryVerified(repo, id, userId); err != nil {
			logger.Log("confirmMicroDeposits", fmt.Sprintf("problem marking depository as Verified: %v", err), "userId", userId)
			return
		}

		// 200 - Micro deposits verified
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
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
	if len(guessAmounts) != len(microDeposits) || len(guessAmounts) == 0 {
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

// getMicroDepositCursor returns a microDepositCursor for iterating through micro-deposits in ascending order (by CreatedAt)
// beginning at the start of the current day.
func (r *sqliteDepositoryRepo) getMicroDepositCursor(batchSize int) *microDepositCursor {
	now := time.Now()
	return &microDepositCursor{
		batchSize: batchSize,
		depRepo:   r,
		newerThan: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
	}
}

// TODO(adam): microDepositCursor (similar to transferCursor for ACH file merging and uploads)
// micro_deposits(depository_id, user_id, amount, file_id, created_at, deleted_at)`
type microDepositCursor struct {
	batchSize int

	depRepo *sqliteDepositoryRepo

	// newerThan represents the minimum (oldest) created_at value to return in the batch.
	// The value starts at today's first instant and progresses towards time.Now() with each
	// batch by being set to the batch's newest time.
	newerThan time.Time
}

type uploadableMicroDeposit struct {
	depositoryId, userId string
	amount               *Amount
	fileId               string
	createdAt            time.Time
}

// Next returns a slice of micro-deposit objects from the current day. Next should be called to process
// all objects for a given day in batches.
func (cur *microDepositCursor) Next() ([]uploadableMicroDeposit, error) {
	query := `select depository_id, user_id, amount, file_id, created_at from micro_deposits where deleted_at is null and merged_filename is null and created_at > ? order by created_at asc limit ?`
	stmt, err := cur.depRepo.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("microDepositCursor.Next: prepare: %v", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(cur.newerThan, cur.batchSize)
	if err != nil {
		return nil, fmt.Errorf("microDepositCursor.Next: query: %v", err)
	}
	defer rows.Close()

	max := cur.newerThan
	var microDeposits []uploadableMicroDeposit
	for rows.Next() {
		var m uploadableMicroDeposit
		var amt string
		if err := rows.Scan(&m.depositoryId, &m.userId, &amt, &m.fileId, &m.createdAt); err != nil {
			return nil, fmt.Errorf("transferCursor.Next: scan: %v", err)
		}
		var amount Amount
		if err := amount.FromString(amt); err != nil {
			return nil, fmt.Errorf("transferCursor.Next: %s Amount from string: %v", amt, err)
		}
		m.amount = &amount
		if m.createdAt.After(max) {
			max = m.createdAt // advance to latest timestamp
		}
		microDeposits = append(microDeposits, m)
	}
	cur.newerThan = max
	return microDeposits, rows.Err()
}

// markMicroDepositAsMerged will set the merged_filename on micro-deposits so they aren't merged into multiple files
// and the file uploaded to the Federal Reserve can be tracked.
func (r *sqliteDepositoryRepo) markMicroDepositAsMerged(filename string, mc uploadableMicroDeposit) error {
	query := `update micro_deposits set merged_filename = ?
where depository_id = ? and file_id = ? and amount = ? and (merged_filename is null or merged_filename = '') and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("markMicroDepositAsMerged: filename=%s: %v", filename, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(filename, mc.depositoryId, mc.fileId, mc.amount.String())
	return err
}

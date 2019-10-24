// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"time"

	accounts "github.com/moov-io/accounts/client"
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/util"

	"github.com/go-kit/kit/log"
)

// ODFIAccount represents the depository account micro-deposts are debited from
type ODFIAccount struct {
	accountNumber string
	routingNumber string
	accountType   AccountType

	client AccountsClient

	mu        sync.Mutex
	accountID string
}

func NewODFIAccount(accountsClient AccountsClient, accountNumber string, routingNumber string, accountType AccountType) *ODFIAccount {
	return &ODFIAccount{
		client:        accountsClient,
		accountNumber: accountNumber,
		routingNumber: routingNumber,
		accountType:   accountType,
	}
}

func (a *ODFIAccount) getID(requestID, userID string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.accountID != "" {
		return a.accountID, nil
	}
	if a.client == nil {
		return "", errors.New("ODFIAccount: nil AccountsClient")
	}

	// Otherwise, make our Accounts HTTP call and grab the ID
	dep := &Depository{
		AccountNumber: a.accountNumber,
		RoutingNumber: a.routingNumber,
		Type:          a.accountType,
	}
	acct, err := a.client.SearchAccounts(requestID, userID, dep)
	if err != nil || (acct == nil || acct.ID == "") {
		return "", fmt.Errorf("ODFIAccount: problem getting accountID: %v", err)
	}
	a.accountID = acct.ID // record account ID for calls later on
	return a.accountID, nil
}

func (a *ODFIAccount) metadata(cfg *config.Config) (*Originator, *Depository) {
	orig := &Originator{
		ID:                "odfi", // TODO(adam): make this NOT querable via db.
		DefaultDepository: DepositoryID("odfi"),
		Identification:    util.Or(cfg.ODFI.Identification, "001"),
		Metadata:          "Moov - paygate micro-deposits",
	}
	dep := &Depository{
		ID:            DepositoryID("odfi"),
		BankName:      util.Or(cfg.ODFI.BankName, "Moov, Inc"),
		Holder:        util.Or(cfg.ODFI.Holder, "Moov, Inc"),
		HolderType:    Individual,
		Type:          a.accountType,
		RoutingNumber: a.routingNumber,
		AccountNumber: a.accountNumber,
		Status:        DepositoryVerified,
	}
	return orig, dep
}

type MicroDeposit struct {
	Amount        Amount
	FileID        string
	TransactionID string
}

func (m MicroDeposit) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Amount Amount `json:"amount"`
	}{
		m.Amount,
	})
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
	return []Amount{*a1, *a2}
}

// initiateMicroDeposits will write micro deposits into the underlying database and kick off the ACH transfer(s).
//
func (r *DepositoryRouter) initiateMicroDeposits() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		w, err := wrapResponseWriter(r.logger, w, httpReq)
		if err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		requestID := moovhttp.GetRequestID(httpReq)

		id, userID := GetDepositoryID(httpReq), moovhttp.GetUserID(httpReq)
		if id == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			// 404 - A depository with the specified ID was not found.
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "depository not found"}`))
			return
		}

		// Check the depository status and confirm it belongs to the user
		dep, err := r.depositoryRepo.GetUserDepository(id, userID)
		if err != nil {
			r.logger.Log("microDeposits", err, "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		if dep == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if dep.Status != DepositoryUnverified {
			err = fmt.Errorf("depository %s in bogus status %s", dep.ID, dep.Status)
			r.logger.Log("microDeposits", err, "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		// Our Depository needs to be Verified so let's submit some micro deposits to it.
		amounts := microDepositAmounts()
		microDeposits, err := r.submitMicroDeposits(userID, requestID, amounts, dep)
		if err != nil {
			err = fmt.Errorf("problem submitting micro-deposits: %v", err)
			r.logger.Log("microDeposits", err, "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		r.logger.Log("microDeposits", fmt.Sprintf("submitted %d micro-deposits for depository=%s", len(microDeposits), dep.ID), "requestID", requestID, "userID", userID)

		// Write micro deposits into our db
		if err := r.depositoryRepo.InitiateMicroDeposits(id, userID, microDeposits); err != nil {
			r.logger.Log("microDeposits", err, "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		r.logger.Log("microDeposits", fmt.Sprintf("stored micro-deposits for depository=%s", dep.ID), "requestID", requestID, "userID", userID)

		w.WriteHeader(http.StatusCreated) // 201 - Micro deposits initiated
		w.Write([]byte("{}"))
	}
}

func postMicroDepositTransaction(logger log.Logger, client AccountsClient, accountID, userID string, lines []transactionLine, requestID string) (*accounts.Transaction, error) {
	var transaction *accounts.Transaction
	var err error
	for i := 0; i < 3; i++ {
		transaction, err = client.PostTransaction(requestID, userID, lines)
		if err == nil {
			break // quit after successful call
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error creating transaction for transfer user=%s: %v", userID, err)
	}
	logger.Log("transfers", fmt.Sprintf("created transaction=%s for user=%s", transaction.ID, userID), "requestID", requestID)
	return transaction, nil
}

func updateMicroDepositsWithTransactionIDs(logger log.Logger, ODFIAccount *ODFIAccount, client AccountsClient, userID string, dep *Depository, microDeposits []*MicroDeposit, sum int, requestID string) ([]*accounts.Transaction, error) {
	if len(microDeposits) != 2 {
		return nil, fmt.Errorf("updateMicroDepositsWithTransactionIDs: got %d micro-deposits", len(microDeposits))
	}
	acct, err := client.SearchAccounts(requestID, userID, dep)
	if err != nil || acct == nil {
		return nil, fmt.Errorf("error reading account user=%s depository=%s: %v", userID, dep.ID, err)
	}
	ODFIAccountID, err := ODFIAccount.getID(requestID, userID)
	if err != nil {
		return nil, fmt.Errorf("posting micro-deposits: %v", err)
	}

	// Submit all micro-deposits
	var transactions []*accounts.Transaction
	for i := range microDeposits {
		lines := []transactionLine{
			{AccountID: acct.ID, Purpose: "ACHCredit", Amount: int32(microDeposits[i].Amount.Int())},
			{AccountID: ODFIAccountID, Purpose: "ACHDebit", Amount: int32(microDeposits[i].Amount.Int())},
		}
		tx, err := postMicroDepositTransaction(logger, client, acct.ID, userID, lines, requestID)
		if err != nil {
			return nil, err // we retried and failed, so just exit early
		}
		microDeposits[i].TransactionID = tx.ID
		transactions = append(transactions, tx)
	}
	// submit the reversal of our micro-deposits
	lines := []transactionLine{
		{AccountID: acct.ID, Purpose: "ACHDebit", Amount: int32(sum)},
		{AccountID: ODFIAccountID, Purpose: "ACHCredit", Amount: int32(sum)},
	}
	tx, err := postMicroDepositTransaction(logger, client, acct.ID, userID, lines, requestID)
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
func (r *DepositoryRouter) submitMicroDeposits(userID string, requestID string, amounts []Amount, dep *Depository) ([]*MicroDeposit, error) {
	odfiOriginator, odfiDepository := r.odfiAccount.metadata(r.config)

	// TODO(adam): reject if user has been failed too much verifying this Depository -- w.WriteHeader(http.StatusConflict)

	var microDeposits []*MicroDeposit
	withdrawAmount, err := NewAmount("USD", "0.00") // TODO(adam): we need to add a test for the higher level endpoint (or see why no test currently fails)
	if err != nil {
		return nil, fmt.Errorf("error with withdrawAmount: %v", err)
	}

	idempotencyKey := base.ID()
	rec := &Receiver{
		ID:       ReceiverID(fmt.Sprintf("%s-micro-deposit-verify", base.ID())),
		Status:   ReceiverVerified, // Something to pass constructACHFile validation logic
		Metadata: dep.Holder,       // Depository holder is getting the micro deposit
	}

	var file *ach.File

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
		}
		req.Receiver, req.ReceiverDepository = rec.ID, dep.ID

		if file == nil {
			xfer := req.asTransfer(string(rec.ID))
			f, err := constructACHFile(string(rec.ID), idempotencyKey, userID, xfer, rec, dep, odfiOriginator, odfiDepository)
			if err != nil {
				err = fmt.Errorf("problem constructing ACH file for userID=%s: %v", userID, err)
				r.logger.Log("microDeposits", err, "requestID", requestID, "userID", userID)
				return nil, err
			}
			file = f
		} else {
			if err := addMicroDeposit(file, amounts[i]); err != nil {
				return nil, err
			}
		}

		// We need to withdraw the micro-deposit from the remote account. To do this simply debit that account by adding another EntryDetail
		if w, err := withdrawAmount.Plus(amounts[i]); err != nil {
			return nil, fmt.Errorf("error adding %v to withdraw amount: %v", amounts[i].String(), err)
		} else {
			withdrawAmount = &w // Plus returns a new instance, so accumulate it
		}

		// If we're on the last micro-deposit then append our withdraw transaction
		if i == len(amounts)-1 {
			req.Type = PullTransfer // pull: withdraw funds

			// Append our withdraw to a file so it's uploaded to the ODFI
			if err := addMicroDepositWithdraw(file, withdrawAmount); err != nil {
				return nil, fmt.Errorf("problem adding withdraw amount: %v", err)
			}
		}
		microDeposits = append(microDeposits, &MicroDeposit{Amount: amounts[i]})

		// Store the Transfer creation as an event
		if err := writeTransferEvent(userID, req, r.eventRepo); err != nil {
			return nil, fmt.Errorf("userID=%s problem writing micro-deposit transfer event: %v", userID, err)
		}
	}

	// Submit the ACH file against moov's ACH service after adding every micro-deposit
	fileID, err := r.achClient.CreateFile(idempotencyKey, file)
	if err != nil {
		err = fmt.Errorf("problem creating ACH file for userID=%s: %v", userID, err)
		r.logger.Log("microDeposits", err, "requestID", requestID, "userID", userID)
		return nil, err
	}
	if err := checkACHFile(r.logger, r.achClient, fileID, userID); err != nil {
		return nil, err
	}
	r.logger.Log("microDeposits", fmt.Sprintf("created ACH file=%s for depository=%s", fileID, dep.ID), "requestID", requestID, "userID", userID)

	for i := range microDeposits {
		microDeposits[i].FileID = fileID
	}

	// Post the transaction against Accounts only if it's enabled (flagged via nil AccountsClient)
	if r.accountsClient != nil {
		transactions, err := updateMicroDepositsWithTransactionIDs(r.logger, r.odfiAccount, r.accountsClient, userID, dep, microDeposits, withdrawAmount.Int(), requestID)
		if err != nil {
			return microDeposits, fmt.Errorf("submitMicroDeposits: error posting to Accounts: %v", err)
		}
		r.logger.Log("microDeposits", fmt.Sprintf("created %d transactions for user=%s micro-deposits", len(transactions), userID), "requestID", requestID)
	}
	return microDeposits, nil
}

func addMicroDeposit(file *ach.File, amt Amount) error {
	if file == nil || len(file.Batches) != 1 || len(file.Batches[0].GetEntries()) != 1 {
		return errors.New("invalid micro-deposit ACH file for deposits")
	}

	// Copy the EntryDetail and replace TransactionCode
	ed := *file.Batches[0].GetEntries()[0] // copy previous EntryDetail
	ed.ID = base.ID()[:8]

	// increment trace number
	if n, _ := strconv.Atoi(ed.TraceNumber); n > 0 {
		ed.TraceNumber = strconv.Itoa(n + 1)
	}

	// use our calculated amount to withdraw all micro-deposits
	ed.Amount = amt.Int()

	// append our new EntryDetail
	file.Batches[0].AddEntry(&ed)

	return nil
}

func addMicroDepositWithdraw(file *ach.File, withdrawAmount *Amount) error {
	// we expect two EntryDetail records (one for each micro-deposit)
	if file == nil || len(file.Batches) != 1 || len(file.Batches[0].GetEntries()) < 1 {
		return errors.New("invalid micro-deposit ACH file for withdraw")
	}

	// We need to adjust ServiceClassCode as this batch has a debit and credit now
	bh := file.Batches[0].GetHeader()
	bh.ServiceClassCode = ach.MixedDebitsAndCredits
	file.Batches[0].SetHeader(bh)

	// Copy the EntryDetail and replace TransactionCode
	entries := file.Batches[0].GetEntries()
	ed := *entries[len(entries)-1] // take last entry detail
	ed.ID = base.ID()[:8]
	// TransactionCodes seem to follow a simple pattern:
	//  37 SavingsDebit -> 32 SavingsCredit
	//  27 CheckingDebit -> 22 CheckingCredit
	ed.TransactionCode -= 5

	// increment trace number
	if n, _ := strconv.Atoi(ed.TraceNumber); n > 0 {
		ed.TraceNumber = strconv.Itoa(n + 1)
	}

	// use our calculated amount to withdraw all micro-deposits
	ed.Amount = withdrawAmount.Int()

	// append our new EntryDetail
	file.Batches[0].AddEntry(&ed)

	return nil
}

type confirmDepositoryRequest struct {
	Amounts []string `json:"amounts"`
}

// confirmMicroDeposits checks our database for a depository's micro deposits (used to validate the user owns the Depository)
// and if successful changes the Depository status to DepositoryVerified.
//
// TODO(adam): Should we allow a Depository to be confirmed before the micro-deposit ACH file is
// uploaded? Technically there's really no way for an end-user to see them before posting, however
// out demo and tests can lookup in Accounts right away and quickly verify the Depository.
func (r *DepositoryRouter) confirmMicroDeposits() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		w, err := wrapResponseWriter(r.logger, w, httpReq)
		if err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		id, userID := GetDepositoryID(httpReq), moovhttp.GetUserID(httpReq)
		if id == "" {
			// 404 - A depository with the specified ID was not found.
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "depository not found"}`))
			return
		}

		// Check the depository status and confirm it belongs to the user
		dep, err := r.depositoryRepo.GetUserDepository(id, userID)
		if err != nil {
			r.logger.Log("confirmMicroDeposits", err, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		if dep.Status != DepositoryUnverified {
			err = fmt.Errorf("depository %s in bogus status %s", dep.ID, dep.Status)
			r.logger.Log("confirmMicroDeposits", err, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		// TODO(adam): if we've failed too many times return '409 - Too many attempts'

		// Read amounts from request JSON
		var req confirmDepositoryRequest
		rr := io.LimitReader(httpReq.Body, maxReadBytes)
		if err := json.NewDecoder(rr).Decode(&req); err != nil {
			r.logger.Log("confirmDepositoryRequest", fmt.Sprintf("problem reading request: %v", err), "userID", userID)
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
			r.logger.Log("confirmMicroDeposits", "no micro-deposit amounts found", "userID", userID)
			// 400 - Invalid Amounts
			moovhttp.Problem(w, errors.New("invalid amounts, found none"))
			return
		}
		if err := r.depositoryRepo.confirmMicroDeposits(id, userID, amounts); err != nil {
			r.logger.Log("confirmMicroDeposits", fmt.Sprintf("problem confirming micro-deposits: %v", err), "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		// Update Depository status
		if err := markDepositoryVerified(r.depositoryRepo, id, userID); err != nil {
			r.logger.Log("confirmMicroDeposits", fmt.Sprintf("problem marking depository as Verified: %v", err), "userID", userID)
			return
		}

		// 200 - Micro deposits verified
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}
}

// GetMicroDeposits will retrieve the micro deposits for a given depository. This endpoint is designed for paygate's admin endpoints.
// If an amount does not parse it will be discardded silently.
func (r *SQLDepositoryRepo) GetMicroDeposits(id DepositoryID) ([]*MicroDeposit, error) {
	query := `select amount, file_id, transaction_id from micro_deposits where depository_id = ?`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return accumulateMicroDeposits(rows)
}

// getMicroDepositsForUser will retrieve the micro deposits for a given depository. If an amount does not parse it will be discardded silently.
func (r *SQLDepositoryRepo) getMicroDepositsForUser(id DepositoryID, userID string) ([]*MicroDeposit, error) {
	query := `select amount, file_id, transaction_id from micro_deposits where user_id = ? and depository_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return accumulateMicroDeposits(rows)
}

func accumulateMicroDeposits(rows *sql.Rows) ([]*MicroDeposit, error) {
	var microDeposits []*MicroDeposit
	for rows.Next() {
		fileID, transactionID := "", ""
		var value string
		if err := rows.Scan(&value, &fileID, &transactionID); err != nil {
			continue
		}

		amt := &Amount{}
		if err := amt.FromString(value); err != nil {
			continue
		}
		microDeposits = append(microDeposits, &MicroDeposit{
			Amount:        *amt,
			FileID:        fileID,
			TransactionID: transactionID,
		})
	}
	return microDeposits, rows.Err()
}

// InitiateMicroDeposits will save the provided []Amount into our database. If amounts have already been saved then
// no new amounts will be added.
func (r *SQLDepositoryRepo) InitiateMicroDeposits(id DepositoryID, userID string, microDeposits []*MicroDeposit) error {
	existing, err := r.getMicroDepositsForUser(id, userID)
	if err != nil || len(existing) > 0 {
		return fmt.Errorf("not initializing more micro deposits, already have %d or got error=%v", len(existing), err)
	}

	// write amounts
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	now, query := time.Now(), `insert into micro_deposits (depository_id, user_id, amount, file_id, transaction_id, created_at) values (?, ?, ?, ?, ?, ?)`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("InitiateMicroDeposits: prepare error=%v rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()

	for i := range microDeposits {
		_, err = stmt.Exec(id, userID, microDeposits[i].Amount.String(), microDeposits[i].FileID, microDeposits[i].TransactionID, now)
		if err != nil {
			return fmt.Errorf("InitiateMicroDeposits: scan error=%v rollback=%v", err, tx.Rollback())
		}
	}

	return tx.Commit()
}

// confirmMicroDeposits will compare the provided guessAmounts against what's been persisted for a user. If the amounts do not match
// or there are a mismatched amount the call will return a non-nil error.
func (r *SQLDepositoryRepo) confirmMicroDeposits(id DepositoryID, userID string, guessAmounts []Amount) error {
	microDeposits, err := r.getMicroDepositsForUser(id, userID)
	if err != nil {
		return fmt.Errorf("unable to confirm micro deposits, got error=%v", err)
	}
	if len(microDeposits) == 0 {
		return errors.New("unable to confirm micro deposits, got 0 micro deposits")
	}

	// Check amounts, all must match
	if len(guessAmounts) != len(microDeposits) || len(guessAmounts) == 0 {
		return fmt.Errorf("incorrect amount of guesses, got %d", len(guessAmounts)) // don't share len(microDeposits), that's an info leak
	}

	found := 0
	for i := range microDeposits {
		for k := range guessAmounts {
			if microDeposits[i].Amount.Equal(guessAmounts[k]) {
				found += 1
				break
			}
		}
	}

	if found != len(microDeposits) {
		return errors.New("incorrect micro deposit guesses")
	}

	return nil
}

// GetMicroDepositCursor returns a microDepositCursor for iterating through micro-deposits in ascending order (by CreatedAt)
// beginning at the start of the current day.
func (r *SQLDepositoryRepo) GetMicroDepositCursor(batchSize int) *MicroDepositCursor {
	now := time.Now()
	return &MicroDepositCursor{
		BatchSize: batchSize,
		DepRepo:   r,
		newerThan: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
	}
}

// MicroDepositCursor allows for iterating through micro-deposits in ascending order (by CreatedAt)
// to merge into files uploaded to an ODFI.
type MicroDepositCursor struct {
	BatchSize int

	DepRepo *SQLDepositoryRepo

	// newerThan represents the minimum (oldest) created_at value to return in the batch.
	// The value starts at today's first instant and progresses towards time.Now() with each
	// batch by being set to the batch's newest time.
	newerThan time.Time
}

type UploadableMicroDeposit struct {
	DepositoryID string
	UserID       string
	Amount       *Amount
	FileID       string
	CreatedAt    time.Time
}

// Next returns a slice of micro-deposit objects from the current day. Next should be called to process
// all objects for a given day in batches.
func (cur *MicroDepositCursor) Next() ([]UploadableMicroDeposit, error) {
	query := `select depository_id, user_id, amount, file_id, created_at from micro_deposits where deleted_at is null and merged_filename is null and created_at > ? order by created_at asc limit ?`
	stmt, err := cur.DepRepo.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("microDepositCursor.Next: prepare: %v", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(cur.newerThan, cur.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("microDepositCursor.Next: query: %v", err)
	}
	defer rows.Close()

	max := cur.newerThan
	var microDeposits []UploadableMicroDeposit
	for rows.Next() {
		var m UploadableMicroDeposit
		var amt string
		if err := rows.Scan(&m.DepositoryID, &m.UserID, &amt, &m.FileID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("transferCursor.Next: scan: %v", err)
		}
		var amount Amount
		if err := amount.FromString(amt); err != nil {
			return nil, fmt.Errorf("transferCursor.Next: %s Amount from string: %v", amt, err)
		}
		m.Amount = &amount
		if m.CreatedAt.After(max) {
			max = m.CreatedAt // advance to latest timestamp
		}
		microDeposits = append(microDeposits, m)
	}
	cur.newerThan = max
	return microDeposits, rows.Err()
}

// MarkMicroDepositAsMerged will set the merged_filename on micro-deposits so they aren't merged into multiple files
// and the file uploaded to the Federal Reserve can be tracked.
func (r *SQLDepositoryRepo) MarkMicroDepositAsMerged(filename string, mc UploadableMicroDeposit) error {
	query := `update micro_deposits set merged_filename = ?
where depository_id = ? and file_id = ? and amount = ? and (merged_filename is null or merged_filename = '') and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("MarkMicroDepositAsMerged: filename=%s: %v", filename, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(filename, mc.DepositoryID, mc.FileID, mc.Amount.String())
	return err
}

func (r *SQLDepositoryRepo) LookupMicroDepositFromReturn(id DepositoryID, amount *Amount) (*MicroDeposit, error) {
	query := `select file_id from micro_deposits where depository_id = ? and amount = ? and deleted_at is null order by created_at desc limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("LookupMicroDepositFromReturn prepare: %v", err)
	}
	defer stmt.Close()

	var fileID string
	if err := stmt.QueryRow(id, amount.String()).Scan(&fileID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("LookupMicroDepositFromReturn scan: %v", err)
	}
	if string(fileID) != "" {
		return &MicroDeposit{Amount: *amount, FileID: fileID}, nil
	}
	return nil, nil
}

// SetReturnCode will write the given returnCode (e.g. "R14") onto the row for one of a Depository's micro-deposit
func (r *SQLDepositoryRepo) SetReturnCode(id DepositoryID, amount Amount, returnCode string) error {
	query := `update micro_deposits set return_code = ? where depository_id = ? and amount = ? and return_code = '' and deleted_at is null;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(returnCode, id, amount.String())
	return err
}

func (r *SQLDepositoryRepo) getMicroDepositReturnCodes(id DepositoryID) []*ach.ReturnCode {
	query := `select distinct md.return_code from micro_deposits as md
inner join depositories as deps on md.depository_id = deps.depository_id
where md.depository_id = ? and deps.status = ? and md.return_code <> '' and md.deleted_at is null and deps.deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil
	}

	rows, err := stmt.Query(id, DepositoryRejected)
	if err != nil {
		return nil
	}
	defer rows.Close()

	returnCodes := make(map[string]*ach.ReturnCode)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil
		}
		if _, exists := returnCodes[code]; !exists {
			returnCodes[code] = ach.LookupReturnCode(code)
		}
	}

	var codes []*ach.ReturnCode
	for k := range returnCodes {
		codes = append(codes, returnCodes[k])
	}
	return codes
}

func ReadMergedFilename(repo *SQLDepositoryRepo, amount *Amount, id DepositoryID) (string, error) {
	query := `select merged_filename from micro_deposits where amount = ? and depository_id = ? limit 1;`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	var mergedFilename string
	if err := stmt.QueryRow(amount.String(), id).Scan(&mergedFilename); err != nil {
		return "", err
	}
	return mergedFilename, nil
}

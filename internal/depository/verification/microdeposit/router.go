// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/gateways"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	microDepositsInitiated = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "micro_deposits_initiated",
		Help: "Counter of micro-deposits initiated against depositories",
	}, []string{"destination"})

	microDepositsConfirmed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "micro_deposits_confirmed",
		Help: "Counter of micro-deposits confirmed for a depository",
	}, []string{"destination"})
)

type Router struct {
	logger log.Logger

	odfiAccount *ODFIAccount
	attempter   attempter

	accountsClient accounts.Client

	repo           Repository
	depositoryRepo depository.Repository
	eventRepo      events.Repository
	gatewayRepo    gateways.Repository
}

func NewRouter(
	logger log.Logger,
	odfiAccount *ODFIAccount,
	attempter attempter,
	accountsClient accounts.Client,
	depRepo depository.Repository,
	eventRepo events.Repository,
	gatewayRepo gateways.Repository,
	microDepositRepo Repository,
) *Router {
	return &Router{
		logger:         logger,
		odfiAccount:    odfiAccount,
		attempter:      attempter,
		accountsClient: accountsClient,
		depositoryRepo: depRepo,
		eventRepo:      eventRepo,
		gatewayRepo:    gatewayRepo,
		repo:           microDepositRepo,
	}
}

func (r *Router) RegisterRoutes(router *mux.Router) {
	router.Methods("POST").Path("/depositories/{depositoryId}/micro-deposits").HandlerFunc(r.initiateMicroDeposits())
	router.Methods("POST").Path("/depositories/{depositoryId}/micro-deposits/confirm").HandlerFunc(r.confirmMicroDeposits())
}

// initiateMicroDeposits will write micro deposits into the underlying database and kick off the ACH transfer(s).
func (r *Router) initiateMicroDeposits() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		depID := depository.GetID(httpReq)
		if depID == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			// 404 - A depository with the specified ID was not found.
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "no depositoryId found"}`))
			return
		}

		// Check the depository status and confirm it belongs to the user
		dep, err := r.depositoryRepo.GetUserDepository(depID, responder.XUserID)
		if err != nil {
			responder.Log("microDeposits", err)
			moovhttp.Problem(w, err)
			return
		}
		if dep == nil {
			moovhttp.Problem(w, fmt.Errorf("initiate micro-deposits: depository=%s not found", depID))
			return
		}
		// dep.Keeper = r.keeper // TODO(adam): we need to copy this over
		if dep.Status != model.DepositoryUnverified {
			err = fmt.Errorf("depository %s in bogus status %s", dep.ID, dep.Status)
			responder.Log("microDeposits", err)
			moovhttp.Problem(w, err)
			return
		}
		if r.attempter != nil {
			if !r.attempter.Available(dep.ID) {
				moovhttp.Problem(w, errors.New("no micro-deposit attempts available"))
				return
			}
		}

		// Our Depository needs to be Verified so let's submit some micro deposits to it.
		amounts := microDepositAmounts()
		microDeposits, err := r.submitMicroDeposits(responder.XUserID, responder.XRequestID, amounts, dep)
		if err != nil {
			err = fmt.Errorf("problem submitting micro-deposits: %v", err)
			responder.Log("microDeposits", err)
			moovhttp.Problem(w, err)
			return
		}
		responder.Log("microDeposits", fmt.Sprintf("submitted %d micro-deposits for depository=%s", len(microDeposits), dep.ID))

		// Write micro deposits into our db
		if err := r.repo.InitiateMicroDeposits(depID, responder.XUserID, microDeposits); err != nil {
			responder.Log("microDeposits", err)
			moovhttp.Problem(w, err)
			return
		}
		responder.Log("microDeposits", fmt.Sprintf("stored micro-deposits for depository=%s", dep.ID))

		microDepositsInitiated.With("destination", dep.RoutingNumber).Add(1)

		w.WriteHeader(http.StatusCreated) // 201 - Micro deposits initiated
		w.Write([]byte("{}"))
	}
}

func postMicroDepositTransaction(logger log.Logger, client accounts.Client, accountID string, userID id.User, lines []accounts.TransactionLine, requestID string) (*accounts.Transaction, error) {
	if client == nil {
		return nil, errors.New("nil Accounts client")
	}

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

func updateMicroDepositsWithTransactionIDs(logger log.Logger, ODFIAccount *ODFIAccount, client accounts.Client, userID id.User, dep *model.Depository, microDeposits []*Credit, sum int, requestID string) ([]*accounts.Transaction, error) {
	if client == nil {
		return nil, errors.New("nil Accounts client")
	}
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
		lines := []accounts.TransactionLine{
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
	lines := []accounts.TransactionLine{
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

func stringifyAmounts(amounts []model.Amount) string {
	buf := ""
	for i := range amounts {
		buf += fmt.Sprintf("%s,", amounts[i].String())
	}
	return strings.TrimSuffix(buf, ",")
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
func (r *Router) submitMicroDeposits(userID id.User, requestID string, amounts []model.Amount, dep *model.Depository) ([]*Credit, error) {
	odfiOriginator, odfiDepository := r.odfiAccount.metadata()
	if odfiOriginator == nil || odfiDepository == nil {
		return nil, errors.New("unable to find ODFI originator or depository")
	}

	if r.attempter != nil {
		if !r.attempter.Available(dep.ID) {
			return nil, errors.New("no micro-deposit attempts available")
		}
		if err := r.attempter.Record(dep.ID, stringifyAmounts(amounts)); err != nil {
			return nil, errors.New("unable to record micro-deposits")
		}
	}

	var microDeposits []*Credit
	debitAmount, err := model.NewAmount("USD", "0.00")
	if err != nil {
		return nil, fmt.Errorf("error with debitAmount: %v", err)
	}

	rec := &model.Receiver{
		ID:       model.ReceiverID(fmt.Sprintf("%s-micro-deposit-verify", base.ID())),
		Status:   model.ReceiverVerified, // Something to pass constructACHFile validation logic
		Metadata: dep.Holder,             // Depository holder is getting the micro deposit
	}

	gateway, err := r.gatewayRepo.GetUserGateway(userID)
	if gateway == nil || err != nil {
		return nil, fmt.Errorf("missing Gateway: %v", err)
	}

	var file *ach.File
	for i := range amounts {
		xfer := &model.Transfer{
			ID:                     id.Transfer(base.ID()),
			Amount:                 amounts[i],
			Originator:             odfiOriginator.ID, // e.g. Moov, Inc
			OriginatorDepository:   odfiDepository.ID,
			Description:            fmt.Sprintf("%s micro-deposit verification", odfiDepository.BankName),
			StandardEntryClassCode: ach.PPD,
			Status:                 model.TransferPending,
			UserID:                 userID.String(),
			PPDDetail: &model.PPDDetail{
				PaymentInformation: "micro-deposit",
			},
		}
		// micro-deposits must balance, the 3rd amount is the other two's sum
		if i == 0 || i == 1 {
			xfer.Type = model.PushTransfer
		}
		xfer.Receiver, xfer.ReceiverDepository = rec.ID, dep.ID

		if file == nil {
			// f, err := remoteach.ConstructFile(string(rec.ID), idempotencyKey, gateway, xfer, rec, dep, odfiOriginator, odfiDepository)
			// if err != nil {
			// 	err = fmt.Errorf("problem constructing ACH file for userID=%s: %v", userID, err)
			// 	r.logger.Log("microDeposits", err, "requestID", requestID, "userID", userID)
			// 	return nil, err
			// }
			// file = f
		} else {
			if err := addMicroDeposit(file, amounts[i]); err != nil {
				return nil, err
			}
		}

		// We need to debit the micro-deposit from the remote account. To do this simply debit that account by adding another EntryDetail
		if w, err := debitAmount.Plus(amounts[i]); err != nil {
			return nil, fmt.Errorf("error adding %v to debit amount: %v", amounts[i].String(), err)
		} else {
			debitAmount = &w // Plus returns a new instance, so accumulate it
		}

		// If we're on the last micro-deposit then append our debit transaction
		if i == len(amounts)-1 {
			xfer.Type = model.PullTransfer // pull: debit funds

			// Append our debit to a file so it's uploaded to the ODFI
			if err := addMicroDepositDebit(file, debitAmount); err != nil {
				return nil, fmt.Errorf("problem adding debit amount: %v", err)
			}
		}
		microDeposits = append(microDeposits, &Credit{Amount: amounts[i]})

		// Store the Transfer creation as an event
		if err := r.eventRepo.WriteEvent(userID, &events.Event{
			ID:      events.EventID(base.ID()),
			Topic:   fmt.Sprintf("%s transfer to %s", xfer.Type, xfer.Description),
			Message: xfer.Description,
			Type:    events.TransferEvent,
		}); err != nil {
			return nil, fmt.Errorf("userID=%s problem writing micro-deposit transfer event: %v", userID, err)
		}
	}

	// Submit the ACH file against moov's ACH service after adding every micro-deposit
	// fileID, err := r.achClient.CreateFile(idempotencyKey, file)
	// if err != nil {
	// 	err = fmt.Errorf("problem creating ACH file for userID=%s: %v", userID, err)
	// 	r.logger.Log("microDeposits", err, "requestID", requestID, "userID", userID)
	// 	return nil, err
	// }
	// if err := remoteach.CheckFile(r.logger, r.achClient, fileID, userID); err != nil {
	// 	return nil, err
	// }
	// r.logger.Log("microDeposits", fmt.Sprintf("created ACH file=%s for depository=%s", fileID, dep.ID), "requestID", requestID, "userID", userID)

	// for i := range microDeposits {
	// 	microDeposits[i].FileID = fileID
	// }

	// Post the transaction against Accounts only if it's enabled (flagged via nil AccountsClient)
	if r.accountsClient != nil {
		transactions, err := updateMicroDepositsWithTransactionIDs(r.logger, r.odfiAccount, r.accountsClient, userID, dep, microDeposits, debitAmount.Int(), requestID)
		if err != nil {
			return microDeposits, fmt.Errorf("submitMicroDeposits: error posting to Accounts: %v", err)
		}
		r.logger.Log("microDeposits", fmt.Sprintf("created %d transactions for user=%s micro-deposits", len(transactions), userID), "requestID", requestID)
	}
	return microDeposits, nil
}

func addMicroDeposit(file *ach.File, amt model.Amount) error {
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

	// use our calculated amount to debit all micro-deposits
	ed.Amount = amt.Int()

	// append our new EntryDetail
	file.Batches[0].AddEntry(&ed)

	return nil
}

func addMicroDepositDebit(file *ach.File, debitAmount *model.Amount) error {
	// we expect two EntryDetail records (one for each micro-deposit)
	if file == nil || len(file.Batches) != 1 || len(file.Batches[0].GetEntries()) < 1 {
		return errors.New("invalid micro-deposit ACH file for debit")
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

	// use our calculated amount to debit all micro-deposits
	ed.Amount = debitAmount.Int()

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
func (r *Router) confirmMicroDeposits() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		depID := depository.GetID(httpReq)
		if depID == "" {
			// 404 - A depository with the specified ID was not found.
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "depository not found"}`))
			return
		}

		// Check the depository status and confirm it belongs to the user
		dep, err := r.depositoryRepo.GetUserDepository(depID, responder.XUserID)
		if err != nil {
			responder.Log("confirmMicroDeposits", err)
			responder.Problem(err)
			return
		}
		if dep.Status != model.DepositoryUnverified {
			err = fmt.Errorf("depository %s in bogus status %s", dep.ID, dep.Status)
			responder.Log("confirmMicroDeposits", err)
			responder.Problem(err)
			return
		}
		if r.attempter != nil {
			if !r.attempter.Available(dep.ID) {
				responder.Problem(errors.New("no micro-deposit attempts available"))
				return
			}
		}

		// Read amounts from request JSON
		var req confirmDepositoryRequest
		if err := json.NewDecoder(route.Read(httpReq.Body)).Decode(&req); err != nil {
			responder.Log("confirmDepositoryRequest", fmt.Sprintf("problem reading request: %v", err))
			responder.Problem(err)
			return
		}

		var amounts []model.Amount
		for i := range req.Amounts {
			amt := &model.Amount{}
			if err := amt.FromString(req.Amounts[i]); err != nil {
				continue
			}
			amounts = append(amounts, *amt)
		}
		if len(amounts) == 0 {
			responder.Log("confirmMicroDeposits", "no micro-deposit amounts found")
			// 400 - Invalid Amounts
			responder.Problem(errors.New("invalid amounts, found none"))
			return
		}
		if err := r.repo.confirmMicroDeposits(depID, responder.XUserID, amounts); err != nil {
			responder.Log("confirmMicroDeposits", fmt.Sprintf("problem confirming micro-deposits: %v", err))
			responder.Problem(err)
			return
		}

		// Update Depository status
		if err := markDepositoryVerified(r.depositoryRepo, depID, responder.XUserID); err != nil {
			responder.Log("confirmMicroDeposits", fmt.Sprintf("problem marking depository as Verified: %v", err))
			return
		}

		microDepositsConfirmed.With("destination", dep.RoutingNumber).Add(1)

		// 200 - Micro deposits verified
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}
}

func markDepositoryVerified(repo depository.Repository, depID id.Depository, userID id.User) error {
	dep, err := repo.GetUserDepository(depID, userID)
	if err != nil {
		return fmt.Errorf("markDepositoryVerified: depository %v (userID=%v): %v", depID, userID, err)
	}
	dep.Status = model.DepositoryVerified
	return repo.UpsertUserDepository(userID, dep)
}

func accumulateMicroDeposits(rows *sql.Rows) ([]*Credit, error) {
	var microDeposits []*Credit
	for rows.Next() {
		fileID, transactionID := "", ""
		var value string
		if err := rows.Scan(&value, &fileID, &transactionID); err != nil {
			continue
		}

		amt := &model.Amount{}
		if err := amt.FromString(value); err != nil {
			continue
		}
		microDeposits = append(microDeposits, &Credit{
			Amount:        *amt,
			FileID:        fileID,
			TransactionID: transactionID,
		})
	}
	return microDeposits, rows.Err()
}

func ReadMergedFilename(repo *SQLRepo, amount *model.Amount, id id.Depository) (string, error) {
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

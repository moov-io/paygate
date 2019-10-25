// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	accounts "github.com/moov-io/accounts/client"
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/base/idempotent"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

var (
	traceNumberSource = rand.NewSource(time.Now().Unix())
)

type TransferID string

func (id TransferID) Equal(s string) bool {
	return strings.EqualFold(string(id), s)
}

type Transfer struct {
	// ID is a unique string representing this Transfer.
	ID TransferID `json:"id"`

	// Type determines if this is a Push or Pull transfer
	Type TransferType `json:"transferType"`

	// Amount is the country currency and quantity
	Amount Amount `json:"amount"`

	// Originator object associated with this transaction
	Originator OriginatorID `json:"originator"`

	// OriginatorDepository is the Depository associated with this transaction
	OriginatorDepository DepositoryID `json:"originatorDepository"`

	// Receiver is the Receiver associated with this transaction
	Receiver ReceiverID `json:"receiver"`

	// ReceiverDepository is the DepositoryID associated with this transaction
	ReceiverDepository DepositoryID `json:"receiverDepository"`

	// Description is a brief summary of the transaction that may appear on the receiving entity’s financial statement
	Description string `json:"description"`

	// StandardEntryClassCode code will be generated based on Receiver type
	StandardEntryClassCode string `json:"standardEntryClassCode"`

	// Status defines the current state of the Transfer
	Status TransferStatus `json:"status"`

	// SameDay indicates that the transfer should be processed the same day if possible.
	SameDay bool `json:"sameDay"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// CCDDetail is an optional struct which enables sending CCD ACH transfers.
	CCDDetail *CCDDetail `json:"CCDDetail,omitempty"`

	// IATDetail is an optional struct which enables sending IAT ACH transfers.
	IATDetail *IATDetail `json:"IATDetail,omitempty"`

	// TELDetail is an optional struct which enables sending TEL ACH transfers.
	TELDetail *TELDetail `json:"TELDetail,omitempty"`

	// WEBDetail is an optional struct which enables sending WEB ACH transfers.
	WEBDetail *WEBDetail `json:"WEBDetail,omitempty"`

	// ReturnCode is an optional struct representing why this Transfer was returned by the RDFI
	ReturnCode *ach.ReturnCode `json:"returnCode"`

	// Hidden fields (populated in LookupTransferFromReturn) which aren't marshaled
	TransactionID string `json:"-"`
	UserID        string `json:"-"`
}

func (t *Transfer) validate() error {
	if t == nil {
		return errors.New("nil Transfer")
	}
	if err := t.Amount.Validate(); err != nil {
		return err
	}
	if err := t.Status.validate(); err != nil {
		return err
	}
	if t.Description == "" {
		return errors.New("transfer: missing description")
	}
	return nil
}

type transferRequest struct {
	Type                   TransferType `json:"transferType"`
	Amount                 Amount       `json:"amount"`
	Originator             OriginatorID `json:"originator"`
	OriginatorDepository   DepositoryID `json:"originatorDepository"`
	Receiver               ReceiverID   `json:"receiver"`
	ReceiverDepository     DepositoryID `json:"receiverDepository"`
	Description            string       `json:"description,omitempty"`
	StandardEntryClassCode string       `json:"standardEntryClassCode"`
	SameDay                bool         `json:"sameDay,omitempty"`

	CCDDetail *CCDDetail `json:"CCDDetail,omitempty"`
	IATDetail *IATDetail `json:"IATDetail,omitempty"`
	TELDetail *TELDetail `json:"TELDetail,omitempty"`
	WEBDetail *WEBDetail `json:"WEBDetail,omitempty"`

	// Internal fields for auditing and tracing
	fileID        string
	transactionID string
}

func (r transferRequest) missingFields() error {
	var missing []string
	check := func(name, s string) {
		if s == "" {
			missing = append(missing, name)
		}
	}

	check("transferType", string(r.Type))
	check("originator", string(r.Originator))
	check("originatorDepository", string(r.OriginatorDepository))
	check("receiver", string(r.Receiver))
	check("receiverDepository", string(r.ReceiverDepository))
	check("standardEntryClassCode", string(r.StandardEntryClassCode))

	if len(missing) > 0 {
		return fmt.Errorf("missing %s JSON field(s)", strings.Join(missing, ", "))
	}
	return nil
}

func (r transferRequest) asTransfer(id string) *Transfer {
	xfer := &Transfer{
		ID:                     TransferID(id),
		Type:                   r.Type,
		Amount:                 r.Amount,
		Originator:             r.Originator,
		OriginatorDepository:   r.OriginatorDepository,
		Receiver:               r.Receiver,
		ReceiverDepository:     r.ReceiverDepository,
		Description:            r.Description,
		StandardEntryClassCode: r.StandardEntryClassCode,
		Status:                 TransferPending,
		SameDay:                r.SameDay,
		Created:                base.Now(),
	}
	// Copy along the YYYDetail sub-object for specific SEC codes
	// where we expect one in the JSON request body.
	switch xfer.StandardEntryClassCode {
	case ach.CCD:
		xfer.CCDDetail = r.CCDDetail
	case ach.IAT:
		xfer.IATDetail = r.IATDetail
	case ach.TEL:
		xfer.TELDetail = r.TELDetail
	case ach.WEB:
		xfer.WEBDetail = r.WEBDetail
	}
	return xfer
}

type TransferType string

const (
	PushTransfer TransferType = "push"
	PullTransfer TransferType = "pull"
)

func (tt TransferType) validate() error {
	switch tt {
	case PushTransfer, PullTransfer:
		return nil
	default:
		return fmt.Errorf("TransferType(%s) is invalid", tt)
	}
}

func (tt *TransferType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*tt = TransferType(strings.ToLower(s))
	if err := tt.validate(); err != nil {
		return err
	}
	return nil
}

type TransferStatus string

const (
	TransferCanceled  TransferStatus = "canceled"
	TransferFailed    TransferStatus = "failed"
	TransferPending   TransferStatus = "pending"
	TransferProcessed TransferStatus = "processed"
	TransferReclaimed TransferStatus = "reclaimed"
)

func (ts TransferStatus) Equal(other TransferStatus) bool {
	return strings.EqualFold(string(ts), string(other))
}

func (ts TransferStatus) validate() error {
	switch ts {
	case TransferCanceled, TransferFailed, TransferPending, TransferProcessed, TransferReclaimed:
		return nil
	default:
		return fmt.Errorf("TransferStatus(%s) is invalid", ts)
	}
}

func (ts *TransferStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*ts = TransferStatus(strings.ToLower(s))
	if err := ts.validate(); err != nil {
		return err
	}
	return nil
}

type TransferRouter struct {
	logger log.Logger

	depRepo            DepositoryRepository
	eventRepo          EventRepository
	receiverRepository receiverRepository
	origRepo           originatorRepository
	transferRepo       TransferRepository

	achClientFactory func(userID string) *achclient.ACH

	accountsClient AccountsClient
}

func NewTransferRouter(
	logger log.Logger,
	depositoryRepo DepositoryRepository,
	eventRepo EventRepository,
	receiverRepo receiverRepository,
	originatorsRepo originatorRepository,
	transferRepo TransferRepository,
	achClientFactory func(userID string) *achclient.ACH,
	accountsClient AccountsClient,
) *TransferRouter {
	return &TransferRouter{
		logger:             logger,
		depRepo:            depositoryRepo,
		eventRepo:          eventRepo,
		receiverRepository: receiverRepo,
		origRepo:           originatorsRepo,
		transferRepo:       transferRepo,
		achClientFactory:   achClientFactory,
		accountsClient:     accountsClient,
	}
}

func (c *TransferRouter) RegisterRoutes(router *mux.Router) {
	router.Methods("GET").Path("/transfers").HandlerFunc(c.getUserTransfers())
	router.Methods("GET").Path("/transfers/{transferId}").HandlerFunc(c.getUserTransfer())

	router.Methods("POST").Path("/transfers").HandlerFunc(c.createUserTransfers())
	router.Methods("POST").Path("/transfers/batch").HandlerFunc(c.createUserTransfers())

	router.Methods("DELETE").Path("/transfers/{transferId}").HandlerFunc(c.deleteUserTransfer())

	router.Methods("GET").Path("/transfers/{transferId}/events").HandlerFunc(c.getUserTransferEvents())
	router.Methods("POST").Path("/transfers/{transferId}/failed").HandlerFunc(c.validateUserTransfer())
	router.Methods("POST").Path("/transfers/{transferId}/files").HandlerFunc(c.getUserTransferFiles())
}

func getTransferID(r *http.Request) TransferID {
	vars := mux.Vars(r)
	v, ok := vars["transferId"]
	if ok {
		return TransferID(v)
	}
	return TransferID("")
}

func (c *TransferRouter) getUserTransfers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(c.logger, w, r)
		if err != nil {
			return
		}

		requestID, userID := moovhttp.GetRequestID(r), moovhttp.GetUserID(r)
		transfers, err := c.transferRepo.getUserTransfers(userID)
		if err != nil {
			c.logger.Log("transfers", fmt.Sprintf("error getting user transfers: %v", err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(transfers)
	}
}

func (c *TransferRouter) getUserTransfer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(c.logger, w, r)
		if err != nil {
			return
		}

		id, userID := getTransferID(r), moovhttp.GetUserID(r)
		requestID := moovhttp.GetRequestID(r)
		transfer, err := c.transferRepo.getUserTransfer(id, userID)
		if err != nil {
			c.logger.Log("transfers", fmt.Sprintf("error reading transfer=%s: %v", id, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(transfer)
	}
}

// readTransferRequests will attempt to parse the incoming body as either a transferRequest or []transferRequest.
// If no requests were read a non-nil error is returned.
func readTransferRequests(r *http.Request) ([]*transferRequest, error) {
	bs, err := read(r.Body)
	if err != nil {
		return nil, err
	}

	var req transferRequest
	var requests []*transferRequest
	if err := json.Unmarshal(bs, &req); err != nil {
		// failed, but try []transferRequest
		if err := json.Unmarshal(bs, &requests); err != nil {
			return nil, err
		}
	} else {
		if err := req.missingFields(); err != nil {
			return nil, err
		}
		requests = append(requests, &req)
	}
	if len(requests) == 0 {
		return nil, errors.New("no Transfer request objects found")
	}
	return requests, nil
}

func (c *TransferRouter) createUserTransfers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(c.logger, w, r)
		if err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		requests, err := readTransferRequests(r)
		if err != nil {
			fmt.Printf("A: %v\n", err)
			moovhttp.Problem(w, err)
			return
		}

		userID, requestID := moovhttp.GetUserID(r), moovhttp.GetRequestID(r)
		ach := c.achClientFactory(userID)

		// Carry over any incoming idempotency key and set one otherwise
		idempotencyKey := idempotent.Header(r)
		if idempotencyKey == "" {
			idempotencyKey = base.ID()
		}

		for i := range requests {
			id, req := base.ID(), requests[i]
			if err := req.missingFields(); err != nil {
				moovhttp.Problem(w, err)
				return
			}

			// Grab and validate objects required for this transfer.
			receiver, receiverDep, orig, origDep, err := getTransferObjects(req, userID, c.depRepo, c.receiverRepository, c.origRepo)
			if err != nil {
				objects := fmt.Sprintf("receiver=%v, receiverDep=%v, orig=%v, origDep=%v, err: %v", receiver, receiverDep, orig, origDep, err)
				c.logger.Log("transfers", fmt.Sprintf("Unable to find all objects during transfer create for user_id=%s, %s", userID, objects))

				// Respond back to user
				moovhttp.Problem(w, fmt.Errorf("missing data to create transfer: %s", err))
				return
			}

			// Post the Transfer's transaction against the Accounts
			var transactionID string
			if c.accountsClient != nil {
				tx, err := c.postAccountTransaction(userID, origDep, receiverDep, req.Amount, req.Type, requestID)
				if err != nil {
					c.logger.Log("transfers", err.Error())
					moovhttp.Problem(w, err)
					return
				}
				transactionID = tx.ID
			}

			// Save Transfer object
			transfer := req.asTransfer(id)
			file, err := constructACHFile(id, idempotencyKey, userID, transfer, receiver, receiverDep, orig, origDep)
			if err != nil {
				moovhttp.Problem(w, err)
				return
			}
			fileID, err := ach.CreateFile(idempotencyKey, file)
			if err != nil {
				moovhttp.Problem(w, err)
				return
			}
			if err := checkACHFile(c.logger, ach, fileID, userID); err != nil {
				moovhttp.Problem(w, err)
				return
			}

			// Add internal ID's (fileID, transaction.ID) onto our request so we can store them in our database
			req.fileID = fileID
			req.transactionID = transactionID

			// Write events for our audit/history log
			if err := writeTransferEvent(userID, req, c.eventRepo); err != nil {
				c.logger.Log("transfers", fmt.Sprintf("error writing transfer=%s event: %v", id, err), "requestID", requestID, "userID", userID)
				moovhttp.Problem(w, err)
				return
			}
		}

		// TODO(adam): We still create Transfers if the micro-deposits have been confirmed, but not merged (and uploaded)
		// into an ACH file. Should we check that case in this method and reject Transfers whose Depositories micro-deposts
		// haven't even been merged yet?

		transfers, err := c.transferRepo.createUserTransfers(userID, requests)
		if err != nil {
			c.logger.Log("transfers", fmt.Sprintf("error creating transfers: %v", err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		writeResponse(c.logger, w, len(requests), transfers)
		c.logger.Log("transfers", fmt.Sprintf("Created transfers for user_id=%s request=%s", userID, requestID))
	}
}

// postAccountTransaction will lookup the Accounts for Depositories involved in a transfer and post the
// transaction against them in order to confirm, when possible, sufficient funds and other checks.
func (c *TransferRouter) postAccountTransaction(userID string, origDep *Depository, recDep *Depository, amount Amount, transferType TransferType, requestID string) (*accounts.Transaction, error) {
	// Let's lookup both accounts. Either account can be "external" (meaning of a RoutingNumber Accounts doesn't control).
	// When the routing numbers don't match we can't do much verify the remote account as we likely don't have Account-level access.
	//
	// TODO(adam): What about an FI that handles multiple routing numbers? Should Accounts expose which routing numbers it currently supports?
	receiverAccount, err := c.accountsClient.SearchAccounts(requestID, userID, recDep)
	if err != nil || receiverAccount == nil {
		return nil, fmt.Errorf("error reading account user=%s receiver depository=%s: %v", userID, recDep.ID, err)
	}
	origAccount, err := c.accountsClient.SearchAccounts(requestID, userID, origDep)
	if err != nil || origAccount == nil {
		return nil, fmt.Errorf("error reading account user=%s originator depository=%s: %v", userID, origDep.ID, err)
	}
	// Submit the transactions to Accounts (only after can we go ahead and save off the Transfer)
	transaction, err := c.accountsClient.PostTransaction(requestID, userID, createTransactionLines(origAccount, receiverAccount, amount, transferType))
	if err != nil {
		return nil, fmt.Errorf("error creating transaction for transfer user=%s: %v", userID, err)
	}
	c.logger.Log("transfers", fmt.Sprintf("created transaction=%s for user=%s amount=%s", transaction.ID, userID, amount.String()))
	return transaction, nil
}

func createTransactionLines(orig *accounts.Account, rec *accounts.Account, amount Amount, transferType TransferType) []transactionLine {
	lines := []transactionLine{
		{AccountID: orig.ID, Amount: int32(amount.Int())}, // originator
		{AccountID: rec.ID, Amount: int32(amount.Int())},  // receiver
	}
	if transferType == PullTransfer {
		lines[0].Purpose, lines[1].Purpose = "ACHCredit", "ACHDebit"
	} else {
		lines[0].Purpose, lines[1].Purpose = "ACHDebit", "ACHCredit"
	}
	return lines
}

func (c *TransferRouter) deleteUserTransfer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(c.logger, w, r)
		if err != nil {
			return
		}

		id, userID := getTransferID(r), moovhttp.GetUserID(r)
		requestID := moovhttp.GetRequestID(r)
		transfer, err := c.transferRepo.getUserTransfer(id, userID)
		if err != nil {
			c.logger.Log("transfers", fmt.Sprintf("error reading transfer=%s for deletion: %v", id, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		if transfer.Status != TransferPending {
			moovhttp.Problem(w, fmt.Errorf("a %s transfer can't be deleted", transfer.Status))
			return
		}

		// Delete from our database
		if err := c.transferRepo.deleteUserTransfer(id, userID); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		// Delete from our ACH service
		fileID, err := c.transferRepo.GetFileIDForTransfer(id, userID)
		if err != nil && err != sql.ErrNoRows {
			moovhttp.Problem(w, err)
			return
		}
		if fileID != "" {
			if err := c.achClientFactory(userID).DeleteFile(fileID); err != nil {
				moovhttp.Problem(w, err)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

// POST /transfers/{id}/failed
// 200 - no errors
// 400 - errors, check json
func (c *TransferRouter) validateUserTransfer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(c.logger, w, r)
		if err != nil {
			return
		}

		// Grab the TransferID and userID
		id, userID := getTransferID(r), moovhttp.GetUserID(r)
		requestID := moovhttp.GetRequestID(r)
		fileID, err := c.transferRepo.GetFileIDForTransfer(id, userID)
		if err != nil {
			c.logger.Log("transfers", fmt.Sprintf("error getting fileID for transfer=%s: %v", id, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		if fileID == "" {
			moovhttp.Problem(w, errors.New("transfer not found"))
			return
		}

		// Check our ACH file status/validity
		if err := checkACHFile(c.logger, c.achClientFactory(userID), fileID, userID); err != nil {
			moovhttp.Problem(w, err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

func (c *TransferRouter) getUserTransferFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(c.logger, w, r)
		if err != nil {
			return
		}

		// Grab the TransferID and userID
		id, userID := getTransferID(r), moovhttp.GetUserID(r)
		requestID := moovhttp.GetRequestID(r)
		fileID, err := c.transferRepo.GetFileIDForTransfer(id, userID)
		if err != nil {
			c.logger.Log("transfers", fmt.Sprintf("error reading fileID for transfer=%s: %v", id, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		if fileID == "" {
			moovhttp.Problem(w, errors.New("transfer not found"))
			return
		}

		// Grab Transfer file(s)
		file, err := c.achClientFactory(userID).GetFile(fileID)
		if err != nil {
			c.logger.Log("transfers", fmt.Sprintf("error getting ACH files for transfer=%s: %v", id, err), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]*ach.File{file})
	}
}

func (c *TransferRouter) getUserTransferEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(c.logger, w, r)
		if err != nil {
			return
		}

		id, userID := getTransferID(r), moovhttp.GetUserID(r)

		transfer, err := c.transferRepo.getUserTransfer(id, userID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		events, err := c.eventRepo.getUserTransferEvents(userID, transfer.ID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(events)
	}
}

type TransferRepository interface {
	getUserTransfers(userID string) ([]*Transfer, error)
	getUserTransfer(id TransferID, userID string) (*Transfer, error)
	UpdateTransferStatus(id TransferID, status TransferStatus) error

	GetFileIDForTransfer(id TransferID, userID string) (string, error)

	LookupTransferFromReturn(sec string, amount *Amount, traceNumber string, effectiveEntryDate time.Time) (*Transfer, error)
	SetReturnCode(id TransferID, returnCode string) error

	// GetTransferCursor returns a database cursor for Transfer objects that need to be
	// posted today.
	//
	// We currently default EffectiveEntryDate to tomorrow for any transfer and thus a
	// transfer created today needs to be posted.
	GetTransferCursor(batchSize int, depRepo DepositoryRepository) *TransferCursor
	MarkTransferAsMerged(id TransferID, filename string, traceNumber string) error

	createUserTransfers(userID string, requests []*transferRequest) ([]*Transfer, error)
	deleteUserTransfer(id TransferID, userID string) error
}

func NewTransferRepo(logger log.Logger, db *sql.DB) *SQLTransferRepo {
	return &SQLTransferRepo{log: logger, db: db}
}

type SQLTransferRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *SQLTransferRepo) Close() error {
	return r.db.Close()
}

func (r *SQLTransferRepo) getUserTransfers(userID string) ([]*Transfer, error) {
	query := `select transfer_id from transfers where user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transferIDs []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return nil, fmt.Errorf("getUserTransfers scan: %v", err)
		}
		if row != "" {
			transferIDs = append(transferIDs, row)
		}
	}

	var transfers []*Transfer
	for i := range transferIDs {
		t, err := r.getUserTransfer(TransferID(transferIDs[i]), userID)
		if err == nil && t.ID != "" {
			transfers = append(transfers, t)
		}
	}
	return transfers, rows.Err()
}

func (r *SQLTransferRepo) getUserTransfer(id TransferID, userID string) (*Transfer, error) {
	query := `select transfer_id, type, amount, originator_id, originator_depository, receiver, receiver_depository, description, standard_entry_class_code, status, same_day, return_code, created_at
from transfers
where transfer_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userID)

	transfer := &Transfer{}
	var (
		amt        string
		returnCode *string
		created    time.Time
	)
	err = row.Scan(&transfer.ID, &transfer.Type, &amt, &transfer.Originator, &transfer.OriginatorDepository, &transfer.Receiver, &transfer.ReceiverDepository, &transfer.Description, &transfer.StandardEntryClassCode, &transfer.Status, &transfer.SameDay, &returnCode, &created)
	if err != nil {
		return nil, err
	}
	if returnCode != nil {
		transfer.ReturnCode = ach.LookupReturnCode(*returnCode)
	}
	transfer.Created = base.NewTime(created)
	// parse Amount struct
	if err := transfer.Amount.FromString(amt); err != nil {
		return nil, err
	}
	if transfer.ID == "" {
		return nil, nil // not found
	}
	return transfer, nil
}

func (r *SQLTransferRepo) UpdateTransferStatus(id TransferID, status TransferStatus) error {
	query := `update transfers set status = ? where transfer_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(status, id)
	return err
}

func (r *SQLTransferRepo) GetFileIDForTransfer(id TransferID, userID string) (string, error) {
	query := `select file_id from transfers where transfer_id = ? and user_id = ? and deleted_at is null limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userID)

	var fileID string
	if err := row.Scan(&fileID); err != nil {
		return "", err
	}
	return fileID, nil
}

func (r *SQLTransferRepo) LookupTransferFromReturn(sec string, amount *Amount, traceNumber string, effectiveEntryDate time.Time) (*Transfer, error) {
	// To match returned files we take a few values which are assumed to uniquely identify a Transfer.
	// traceNumber, per NACHA guidelines, should be globally unique (routing number + random value),
	// but we are going to filter to only select Transfers created within a few days of the EffectiveEntryDate
	// to avoid updating really old (or future, I suppose) objects.
	query := `select transfer_id, user_id, transaction_id from transfers
where standard_entry_class_code = ? and amount = ? and trace_number = ? and status = ? and (created_at > ? and created_at < ?) and deleted_at is null limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	transferId, userID, transactionID := "", "", "" // holders for 'select ..'
	min, max := startOfDayAndTomorrow(effectiveEntryDate)
	// Only include Transfer objects within 5 calendar days of the EffectiveEntryDate
	min = min.Add(-5 * 24 * time.Hour)
	max = max.Add(5 * 24 * time.Hour)

	row := stmt.QueryRow(sec, amount.String(), traceNumber, TransferProcessed, min, max)
	if err := row.Scan(&transferId, &userID, &transactionID); err != nil {
		return nil, err
	}

	xfer, err := r.getUserTransfer(TransferID(transferId), userID)
	xfer.TransactionID = transactionID
	xfer.UserID = userID
	return xfer, err
}

func (r *SQLTransferRepo) SetReturnCode(id TransferID, returnCode string) error {
	query := `update transfers set return_code = ? where transfer_id = ? and return_code is null and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(returnCode, id)
	return err
}

func (r *SQLTransferRepo) createUserTransfers(userID string, requests []*transferRequest) ([]*Transfer, error) {
	query := `insert into transfers (transfer_id, user_id, type, amount, originator_id, originator_depository, receiver, receiver_depository, description, standard_entry_class_code, status, same_day, file_id, transaction_id, created_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var transfers []*Transfer

	now := time.Now()
	var status TransferStatus = TransferPending
	for i := range requests {
		req, transferId := requests[i], base.ID()
		xfer := &Transfer{
			ID:                     TransferID(transferId),
			Type:                   req.Type,
			Amount:                 req.Amount,
			Originator:             req.Originator,
			OriginatorDepository:   req.OriginatorDepository,
			Receiver:               req.Receiver,
			ReceiverDepository:     req.ReceiverDepository,
			Description:            req.Description,
			StandardEntryClassCode: req.StandardEntryClassCode,
			Status:                 status,
			SameDay:                req.SameDay,
			Created:                base.NewTime(now),
		}
		if err := xfer.validate(); err != nil {
			return nil, fmt.Errorf("validation failed for transfer Originator=%s, Receiver=%s, Description=%s %v", xfer.Originator, xfer.Receiver, xfer.Description, err)
		}

		// write transfer
		_, err := stmt.Exec(transferId, userID, req.Type, req.Amount.String(), req.Originator, req.OriginatorDepository, req.Receiver, req.ReceiverDepository, req.Description, req.StandardEntryClassCode, status, req.SameDay, req.fileID, req.transactionID, now)
		if err != nil {
			return nil, err
		}
		transfers = append(transfers, xfer)
	}
	return transfers, nil
}

func (r *SQLTransferRepo) deleteUserTransfer(id TransferID, userID string) error {
	query := `update transfers set deleted_at = ? where transfer_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(time.Now(), id, userID)
	return err
}

// TransferCursor allows for iterating through Transfers in ascending order (by CreatedAt)
// to merge into files uploaded to an ODFI.
type TransferCursor struct {
	BatchSize int

	DepRepo      DepositoryRepository
	TransferRepo *SQLTransferRepo

	// newerThan represents the minimum (oldest) created_at value to return in the batch.
	// The value starts at today's first instant and progresses towards time.Now() with each
	// batch by being set to the batch's newest time.
	newerThan time.Time
}

// GroupableTransfer holds metadata of a Transfer used in grouping for generating and merging ACH files
// to be uploaded into the Fed.
type GroupableTransfer struct {
	*Transfer

	// Origin is the ABA routing number of the Originating FI (ODFI)
	// This comes from the Transfer's OriginatorDepository.RoutingNumber
	Origin string

	userID string
}

func (t GroupableTransfer) UserID() string {
	return t.userID
}

// Next returns a slice of Transfer objects from the current day. Next should be called to process
// all objects for a given day in batches.
//
// TODO(adam): should we have a field on transfers for marking when the ACH file is uploaded?
// "after the file is uploaded we mark the items in the DB with the batch number and upload time and update the status" -- Wade
func (cur *TransferCursor) Next() ([]*GroupableTransfer, error) {
	query := `select transfer_id, user_id, created_at from transfers where status = ? and merged_filename is null and created_at > ? and deleted_at is null order by created_at asc limit ?`
	stmt, err := cur.TransferRepo.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("TransferCursor.Next: prepare: %v", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(TransferPending, cur.newerThan, cur.BatchSize) // only Pending transfers
	if err != nil {
		return nil, fmt.Errorf("TransferCursor.Next: query: %v", err)
	}
	defer rows.Close()

	type xfer struct {
		transferId, userID string
		createdAt          time.Time
	}
	var xfers []xfer
	for rows.Next() {
		var xf xfer
		if err := rows.Scan(&xf.transferId, &xf.userID, &xf.createdAt); err != nil {
			return nil, fmt.Errorf("TransferCursor.Next: scan: %v", err)
		}
		if xf.transferId != "" {
			xfers = append(xfers, xf)
		}
	}

	max := cur.newerThan

	var transfers []*GroupableTransfer
	for i := range xfers {
		t, err := cur.TransferRepo.getUserTransfer(TransferID(xfers[i].transferId), xfers[i].userID)
		if err != nil {
			continue
		}
		originDep, err := cur.DepRepo.GetUserDepository(t.OriginatorDepository, xfers[i].userID)
		if err != nil || originDep == nil {
			continue
		}
		transfers = append(transfers, &GroupableTransfer{
			Transfer: t,
			Origin:   originDep.RoutingNumber,
			userID:   xfers[i].userID,
		})
		if xfers[i].createdAt.After(max) {
			max = xfers[i].createdAt // advance max to newest time
		}
	}
	cur.newerThan = max
	return transfers, rows.Err()
}

// GetTransferCursor returns a TransferCursor for iterating through Transfers in ascending order (by CreatedAt)
// beginning at the start of the current day.
func (r *SQLTransferRepo) GetTransferCursor(batchSize int, depRepo DepositoryRepository) *TransferCursor {
	now := time.Now()
	return &TransferCursor{
		BatchSize:    batchSize,
		TransferRepo: r,
		newerThan:    time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		DepRepo:      depRepo,
	}
}

// MarkTransferAsMerged will set the merged_filename on Pending transfers so they aren't merged into multiple files
// and the file uploaded to the FED can be tracked.
func (r *SQLTransferRepo) MarkTransferAsMerged(id TransferID, filename string, traceNumber string) error {
	query := `update transfers set merged_filename = ?, trace_number = ?, status = ?
where status = ? and transfer_id = ? and (merged_filename is null or merged_filename = '') and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("MarkTransferAsMerged: transfer=%s filename=%s: %v", id, filename, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(filename, traceNumber, TransferProcessed, TransferPending, id)
	return err
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

// getTransferObjects performs database lookups to grab all the objects needed to make a transfer.
//
// This method also verifies the status of the Receiver, Receiver Depository and Originator Repository
//
// All return values are either nil or non-nil and the error will be the opposite.
func getTransferObjects(req *transferRequest, userID string, depRepo DepositoryRepository, receiverRepository receiverRepository, origRepo originatorRepository) (*Receiver, *Depository, *Originator, *Depository, error) {
	// Receiver
	receiver, err := receiverRepository.getUserReceiver(req.Receiver, userID)
	if err != nil {
		return nil, nil, nil, nil, errors.New("receiver not found")
	}
	if err := receiver.validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("receiver: %v", err)
	}

	receiverDep, err := depRepo.GetUserDepository(req.ReceiverDepository, userID)
	if err != nil {
		return nil, nil, nil, nil, errors.New("receiver depository not found")
	}
	if receiverDep.Status != DepositoryVerified {
		return nil, nil, nil, nil, fmt.Errorf("receiver depository %s is in status %v", receiverDep.ID, receiverDep.Status)
	}
	if err := receiverDep.validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("receiver depository: %v", err)
	}

	// Originator
	orig, err := origRepo.getUserOriginator(req.Originator, userID)
	if err != nil {
		return nil, nil, nil, nil, errors.New("originator not found")
	}
	if err := orig.validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("originator: %v", err)
	}

	origDep, err := depRepo.GetUserDepository(req.OriginatorDepository, userID)
	if err != nil {
		return nil, nil, nil, nil, errors.New("originator Depository not found")
	}
	if origDep.Status != DepositoryVerified {
		return nil, nil, nil, nil, fmt.Errorf("originator Depository %s is in status %v", origDep.ID, origDep.Status)
	}
	if err := origDep.validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("originator depository: %v", err)
	}

	return receiver, receiverDep, orig, origDep, nil
}

// constructACHFile will take in a Transfer and metadata to build an ACH file which can be submitted against an ACH instance.
func constructACHFile(id, idempotencyKey, userID string, transfer *Transfer, receiver *Receiver, receiverDep *Depository, orig *Originator, origDep *Depository) (*ach.File, error) {
	// TODO(adam): KYC (via Customers) is needed before we validate / reject Receivers
	if transfer.Type == PullTransfer && receiver.Status != ReceiverVerified {
		// TODO(adam): "additional checks" - check Receiver.Status ???
		// https://github.com/moov-io/paygate/issues/18#issuecomment-432066045
		return nil, fmt.Errorf("receiver_id=%s is not Verified user_id=%s", receiver.ID, userID)
	}
	if transfer.Status != TransferPending {
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
		batch, err := createCCDBatch(id, userID, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddBatch(batch)
	case ach.IAT:
		batch, err := createIATBatch(id, userID, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddIATBatch(*batch)
	case ach.PPD:
		batch, err := createPPDBatch(id, userID, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddBatch(batch)
	case ach.TEL:
		batch, err := createTELBatch(id, userID, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddBatch(batch)
	case ach.WEB:
		batch, err := createWEBBatch(id, userID, transfer, receiver, receiverDep, orig, origDep)
		if err != nil {
			return nil, fmt.Errorf("constructACHFile: %s: %v", transfer.StandardEntryClassCode, err)
		}
		file.AddBatch(batch)
	default:
		return nil, fmt.Errorf("unsupported SEC code: %s", transfer.StandardEntryClassCode)
	}
	return file, nil
}

// checkACHFile calls out to our ACH service to build and validate the ACH file,
// "build" involves the ACH service computing some file/batch level totals and checksums.
func checkACHFile(logger log.Logger, client *achclient.ACH, fileID, userID string) error {
	// We don't care about the resposne, just the side-effect build tabulations.
	if _, err := client.GetFileContents(fileID); err != nil && logger != nil {
		logger.Log("transfers", fmt.Sprintf("userID=%s fileID=%s err=%v", userID, fileID, err))
	}
	// ValidateFile will return specific file-level information about what's wrong.
	return client.ValidateFile(fileID)
}

func determineServiceClassCode(t *Transfer) int {
	if t.Type == PushTransfer {
		return ach.CreditsOnly
	}
	return ach.DebitsOnly
}

func determineTransactionCode(t *Transfer, origDep *Depository) int {
	switch {
	case t == nil:
		return 0 // invalid, so we error
	case strings.EqualFold(t.StandardEntryClassCode, ach.TEL):
		if origDep.Type == Checking {
			return ach.CheckingDebit // Debit (withdrawal) to checking account ‘27’
		}
		return ach.SavingsDebit // Debit to savings account ‘37’
	default:
		if origDep.Type == Checking {
			if t.Type == PushTransfer {
				return ach.CheckingCredit
			}
			return ach.CheckingDebit
		} else { // Savings
			if t.Type == PushTransfer {
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

func writeTransferEvent(userID string, req *transferRequest, eventRepo EventRepository) error {
	return eventRepo.writeEvent(userID, &Event{
		ID:      EventID(base.ID()),
		Topic:   fmt.Sprintf("%s transfer to %s", req.Type, req.Description),
		Message: req.Description,
		Type:    TransferEvent,
	})
}

func writeResponse(logger log.Logger, w http.ResponseWriter, reqCount int, transfers []*Transfer) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if reqCount == 1 {
		// don't render surrounding array for single transfer create
		// (it's coming from POST /transfers, not POST /transfers/batch)
		json.NewEncoder(w).Encode(transfers[0])
	} else {
		json.NewEncoder(w).Encode(transfers)
	}
}

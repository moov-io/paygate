// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

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

	// Customer is the Customer associated with this transaction
	Customer CustomerID `json:"customer"`

	// CustomerDepository is the DepositoryID associated with this transaction
	CustomerDepository DepositoryID `json:"customerDepository"`

	// Description is a brief summary of the transaction that may appear on the receiving entity’s financial statement
	Description string `json:"description"`

	// StandardEntryClassCode code will be generated based on Customer type
	StandardEntryClassCode string `json:"standardEntryClassCode"`

	// Status defines the current state of the Transfer
	Status TransferStatus `json:"status"`

	// SameDay indicates that the transfer should be processed the same day if possible.
	SameDay bool `json:"sameDay"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// WEBDetail is an optional struct which enables sending WEB ACH transfers.
	WEBDetail WEBDetail `json:"WEBDetail,omitempty"`

	// IATDetail is an optional struct which enables sending IAT ACH transfers.
	IATDetail IATDetail `json:"IATDetail,omitempty"`
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
		return errors.New("Transfer: missing description")
	}
	return nil
}

type transferRequest struct {
	Type                   TransferType `json:"transferType"`
	Amount                 Amount       `json:"amount"`
	Originator             OriginatorID `json:"originator"`
	OriginatorDepository   DepositoryID `json:"originatorDepository"`
	Customer               CustomerID   `json:"customer"`
	CustomerDepository     DepositoryID `json:"customerDepository"`
	Description            string       `json:"description,omitempty"`
	StandardEntryClassCode string       `json:"standardEntryClassCode"`
	SameDay                bool         `json:"sameDay,omitempty"`
	WEBDetail              WEBDetail    `json:"WEBDetail,omitempty"`
	IATDetail              IATDetail    `json:"IATDetail,omitempty"`

	// ACH service fileId
	fileId string
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
	check("customer", string(r.Customer))
	check("customerDepository", string(r.CustomerDepository))
	check("standardEntryClassCode", string(r.StandardEntryClassCode))

	if len(missing) > 0 {
		return fmt.Errorf("missing %s JSON field(s)", strings.Join(missing, ", "))
	}
	return nil
}

func (r transferRequest) asTransfer(id string) *Transfer {
	return &Transfer{
		ID:                     TransferID(id),
		Type:                   r.Type,
		Amount:                 r.Amount,
		Originator:             r.Originator,
		OriginatorDepository:   r.OriginatorDepository,
		Customer:               r.Customer,
		CustomerDepository:     r.CustomerDepository,
		Description:            r.Description,
		StandardEntryClassCode: r.StandardEntryClassCode,
		Status:                 TransferPending,
		SameDay:                r.SameDay,
		Created:                base.Now(),
	}
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

func addTransfersRoute(r *mux.Router, custRepo customerRepository, depRepo depositoryRepository, eventRepo eventRepository, origRepo originatorRepository, transferRepo transferRepository) {
	r.Methods("GET").Path("/transfers").HandlerFunc(getUserTransfers(transferRepo))
	r.Methods("GET").Path("/transfers/{transferId}").HandlerFunc(getUserTransfer(transferRepo))

	r.Methods("POST").Path("/transfers").HandlerFunc(createUserTransfers(custRepo, depRepo, eventRepo, origRepo, transferRepo))
	r.Methods("POST").Path("/transfers/batch").HandlerFunc(createUserTransfers(custRepo, depRepo, eventRepo, origRepo, transferRepo))

	r.Methods("DELETE").Path("/transfers/{transferId}").HandlerFunc(deleteUserTransfer(transferRepo))

	r.Methods("GET").Path("/transfers/{transferId}/events").HandlerFunc(getUserTransferEvents(eventRepo, transferRepo))
	r.Methods("POST").Path("/transfers/{transferId}/failed").HandlerFunc(validateUserTransfer(transferRepo))
	r.Methods("POST").Path("/transfers/{transferId}/files").HandlerFunc(getUserTransferFiles(transferRepo))
}

func getTransferId(r *http.Request) TransferID {
	vars := mux.Vars(r)
	v, ok := vars["transferId"]
	if ok {
		return TransferID(v)
	}
	return TransferID("")
}

func getUserTransfers(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserTransfers")
		if err != nil {
			return
		}

		userId := moovhttp.GetUserId(r)
		transfers, err := transferRepo.getUserTransfers(userId)
		if err != nil {
			internalError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(transfers); err != nil {
			internalError(w, err)
			return
		}
	}
}

func getUserTransfer(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserTransfer")
		if err != nil {
			return
		}

		id, userId := getTransferId(r), moovhttp.GetUserId(r)
		transfer, err := transferRepo.getUserTransfer(id, userId)
		if err != nil {
			internalError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(transfer); err != nil {
			internalError(w, err)
			return
		}
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

func createUserTransfers(custRepo customerRepository, depRepo depositoryRepository, eventRepo eventRepository, origRepo originatorRepository, transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserTransfers")
		if err != nil {
			return
		}

		idempotencyKey, seen := idempotent.FromRequest(r, inmemIdempot)
		if seen {
			idempotent.SeenBefore(w)
			return
		}

		requests, err := readTransferRequests(r)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		userId, requestId := moovhttp.GetUserId(r), moovhttp.GetRequestId(r)
		ach := achclient.New(userId, logger)

		for i := range requests {
			id, req := nextID(), requests[i]
			if err := req.missingFields(); err != nil {
				moovhttp.Problem(w, err)
				return
			}

			// Grab and validate objects required for this transfer.
			cust, custDep, orig, origDep, err := getTransferObjects(req, userId, custRepo, depRepo, origRepo)
			if err != nil {
				// Internal log
				objects := fmt.Sprintf("cust=%v, custDep=%v, orig=%v, origDep=%v, err: %v", cust, custDep, orig, origDep, err)
				logger.Log("transfers", fmt.Sprintf("Unable to find all objects during transfer create for user_id=%s, %s", userId, objects))

				// Respond back to user
				moovhttp.Problem(w, fmt.Errorf("Missing data to create transfer: %s", err))
				return
			}

			// Save Transfer object
			transfer := req.asTransfer(id)
			fileId, err := createACHFile(ach, id, idempotencyKey, userId, transfer, cust, custDep, orig, origDep)
			if err != nil {
				moovhttp.Problem(w, err)
				return
			}
			if err := checkACHFile(ach, fileId, userId); err != nil {
				moovhttp.Problem(w, err)
				return
			}

			req.fileId = fileId // add fileId onto our request

			// Write events for our audit/history log
			if err := writeTransferEvent(userId, req, eventRepo); err != nil {
				internalError(w, err)
				return
			}
		}

		transfers, err := transferRepo.createUserTransfers(userId, requests)
		if err != nil {
			internalError(w, err)
			return
		}

		writeResponse(w, len(requests), transfers)
		logger.Log("transfers", fmt.Sprintf("Created transfers for user_id=%s request=%s", userId, requestId))
	}
}

func deleteUserTransfer(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserTransfer")
		if err != nil {
			return
		}

		// TODO(adam): Check status? Only allow Pending transfers to be deleted?

		id, userId := getTransferId(r), moovhttp.GetUserId(r)

		// Delete from our ACH service
		fileId, err := transferRepo.getFileIdForTransfer(id, userId)
		if err != nil {
			internalError(w, err)
			return
		}

		ach := achclient.New(userId, logger)
		if err := ach.DeleteFile(fileId); err != nil { // TODO(adam): ignore 404's
			internalError(w, err)
			return
		}

		// Delete from our database
		if err := transferRepo.deleteUserTransfer(id, userId); err != nil {
			internalError(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// POST /transfers/{id}/failed
// 200 - no errors
// 400 - errors, check json
func validateUserTransfer(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "validateUserTransfer")
		if err != nil {
			return
		}

		w.WriteHeader(http.StatusOK) // TODO(adam): implement
	}
}

func getUserTransferFiles(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserTransferFiles")
		if err != nil {
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		w.Write([]byte("files, todo"))
	}
}

func getUserTransferEvents(eventRepo eventRepository, transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserTransferEvents")
		if err != nil {
			return
		}

		id, userId := getTransferId(r), moovhttp.GetUserId(r)

		transfer, err := transferRepo.getUserTransfer(id, userId)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		events, err := eventRepo.getUserTransferEvents(userId, transfer.ID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(events); err != nil {
			internalError(w, err)
			return
		}

	}
}

type transferRepository interface {
	getUserTransfers(userId string) ([]*Transfer, error)
	getUserTransfer(id TransferID, userId string) (*Transfer, error)
	getFileIdForTransfer(id TransferID, userId string) (string, error)

	// getTransferCursor returns a database cursor for Transfer objects that need to be
	// posted today.
	//
	// We currently default EffectiveEntryDate to tomorrow for any transfer and thus a
	// transfer created today needs to be posted.
	//
	// TODO(adam): read EffectiveEntryDate from JSON? I assume people will want to schedule
	// transfers (and we need to store that on the transfers table too).
	getTransferCursor(batchSize int, depRepo depositoryRepository) *transferCursor
	markTransferAsMerged(id TransferID, filename string) error

	createUserTransfers(userId string, requests []*transferRequest) ([]*Transfer, error)
	deleteUserTransfer(id TransferID, userId string) error
}

type sqliteTransferRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *sqliteTransferRepo) close() error {
	return r.db.Close()
}

func (r *sqliteTransferRepo) getUserTransfers(userId string) ([]*Transfer, error) {
	query := `select transfer_id from transfers where user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transferIds []string
	for rows.Next() {
		var row string
		rows.Scan(&row)
		if row != "" {
			transferIds = append(transferIds, row)
		}
	}

	var transfers []*Transfer
	for i := range transferIds {
		t, err := r.getUserTransfer(TransferID(transferIds[i]), userId)
		if err == nil && t.ID != "" {
			transfers = append(transfers, t)
		}
	}
	return transfers, nil
}

func (r *sqliteTransferRepo) getUserTransfer(id TransferID, userId string) (*Transfer, error) {
	query := `select transfer_id, type, amount, originator_id, originator_depository, customer, customer_depository, description, standard_entry_class_code, status, same_day, created_at
from transfers
where transfer_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userId)

	transfer := &Transfer{}
	var (
		amt     string
		created time.Time
	)
	err = row.Scan(&transfer.ID, &transfer.Type, &amt, &transfer.Originator, &transfer.OriginatorDepository, &transfer.Customer, &transfer.CustomerDepository, &transfer.Description, &transfer.StandardEntryClassCode, &transfer.Status, &transfer.SameDay, &created)
	if err != nil {
		return nil, err
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

func (r *sqliteTransferRepo) getFileIdForTransfer(id TransferID, userId string) (string, error) {
	query := `select file_id from transfers where transfer_id = ? and user_id = ? and deleted_at is null limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userId)

	var fileId string
	if err := row.Scan(&fileId); err != nil {
		return "", err
	}
	return fileId, nil
}

func (r *sqliteTransferRepo) createUserTransfers(userId string, requests []*transferRequest) ([]*Transfer, error) {
	query := `insert into transfers (transfer_id, user_id, type, amount, originator_id, originator_depository, customer, customer_depository, description, standard_entry_class_code, status, same_day, file_id, created_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var transfers []*Transfer

	now := time.Now()
	var status TransferStatus = TransferPending
	for i := range requests {
		req, transferId := requests[i], nextID()
		xfer := &Transfer{
			ID:                     TransferID(transferId),
			Type:                   req.Type,
			Amount:                 req.Amount,
			Originator:             req.Originator,
			OriginatorDepository:   req.OriginatorDepository,
			Customer:               req.Customer,
			CustomerDepository:     req.CustomerDepository,
			Description:            req.Description,
			StandardEntryClassCode: req.StandardEntryClassCode,
			Status:                 status,
			SameDay:                req.SameDay,
			Created:                base.NewTime(now),
		}
		if err := xfer.validate(); err != nil {
			return nil, fmt.Errorf("validation failed for transfer Originator=%s, Customer=%s, Description=%s %v", xfer.Originator, xfer.Customer, xfer.Description, err)
		}

		// write transfer
		_, err := stmt.Exec(transferId, userId, req.Type, req.Amount.String(), req.Originator, req.OriginatorDepository, req.Customer, req.CustomerDepository, req.Description, req.StandardEntryClassCode, status, req.SameDay, req.fileId, now)
		if err != nil {
			return nil, err
		}
		transfers = append(transfers, xfer)
	}
	return transfers, nil
}

func (r *sqliteTransferRepo) deleteUserTransfer(id TransferID, userId string) error {
	query := `update transfers set deleted_at = ? where transfer_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(time.Now(), id, userId)
	return err
}

type transferCursor struct {
	batchSize int

	depRepo      depositoryRepository
	transferRepo *sqliteTransferRepo

	// newerThan represents the minimum (oldest) created_at value to return in the batch.
	// The value starts at today's first instant and progresses towards time.Now() with each
	// batch by being set to the batch's newest time.
	newerThan time.Time
}

// groupableTransfer holds metadata of a Transfer used in grouping for generating and merging ACH files
// to be uploaded into the Fed.
type groupableTransfer struct {
	*Transfer

	// destination is the ABA routing number of the destination FI
	// This comes from the Transfers.CustomerDepository.Destination
	destination string

	userId string
}

// Next returns a slice of Transfer objects from the current day. Next should be called to process
// all objects for a given day in batches.
//
// TODO(adam): should we have a field on transfers for marking when the ACH file is uploaded?
// "after the file is uploaded we mark the items in the DB with the batch number and upload time and update the status"
func (cur *transferCursor) Next() ([]*groupableTransfer, error) {
	query := `select transfer_id, user_id, created_at from transfers where status = ? and merged_filename is null and created_at > ? and deleted_at is null order by created_at asc limit ?`
	stmt, err := cur.transferRepo.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(TransferPending, cur.newerThan, cur.batchSize) // only Pending transfers
	if err != nil {
		return nil, err
	}

	type xfer struct {
		transferId, userId string
		createdAt          time.Time
	}
	var xfers []xfer
	for rows.Next() {
		var xf xfer
		rows.Scan(&xf.transferId, &xf.userId, &xf.createdAt)
		if xf.transferId != "" {
			xfers = append(xfers, xf)
		}
	}
	rows.Close()

	max := cur.newerThan

	var transfers []*groupableTransfer
	for i := range xfers {
		t, err := cur.transferRepo.getUserTransfer(TransferID(xfers[i].transferId), xfers[i].userId)
		if err != nil {
			continue // TODO(adam): log ?
		}
		custDep, err := cur.depRepo.getUserDepository(t.CustomerDepository, xfers[i].userId)
		if err != nil || custDep == nil {
			continue // TODO(adam): log ?
		}
		transfers = append(transfers, &groupableTransfer{
			Transfer:    t,
			destination: custDep.RoutingNumber,
			userId:      xfers[i].userId,
		})
		if xfers[i].createdAt.After(max) {
			// advance max to newest time
			max = xfers[i].createdAt
		}
	}
	cur.newerThan = max
	return transfers, nil
}

func (r *sqliteTransferRepo) getTransferCursor(batchSize int, depRepo depositoryRepository) *transferCursor {
	now := time.Now()
	return &transferCursor{
		batchSize:    batchSize,
		transferRepo: r,
		newerThan:    time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		depRepo:      depRepo,
	}
}

// markTransferAsMerged will set the merged_filename on Pending transfers so they aren't merged into multiple files
// and the file uploaded to the FED can be tracked.
func (r *sqliteTransferRepo) markTransferAsMerged(id TransferID, filename string) error {
	query := `update transfers set merged_filename = ? where status = ? and transfer_id = ? and (merged_filename is null or merged_filename = '') and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("markTransferAsMerged: transfer=%s filename=%s: %v", id, filename, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(filename, TransferPending, id)
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
// This method also verifies the status of the Customer, Customer Depository and Originator Repository
//
// All return values are either nil or non-nil and the error will be the opposite.
func getTransferObjects(req *transferRequest, userId string, custRepo customerRepository, depRepo depositoryRepository, origRepo originatorRepository) (*Customer, *Depository, *Originator, *Depository, error) {
	// Customer
	cust, err := custRepo.getUserCustomer(req.Customer, userId)
	if err != nil {
		return nil, nil, nil, nil, errors.New("customer not found")
	}
	if err := cust.validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("customer: %v", err)
	}

	custDep, err := depRepo.getUserDepository(req.CustomerDepository, userId)
	if err != nil {
		return nil, nil, nil, nil, errors.New("customer depository not found")
	}
	if custDep.Status != DepositoryVerified {
		return nil, nil, nil, nil, fmt.Errorf("customer depository %s is in status %v", custDep.ID, custDep.Status)
	}
	if err := custDep.validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("customer depository: %v", err)
	}

	// Originator
	orig, err := origRepo.getUserOriginator(req.Originator, userId)
	if err != nil {
		return nil, nil, nil, nil, errors.New("Originator not found")
	}
	if err := orig.validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("originator: %v", err)
	}

	origDep, err := depRepo.getUserDepository(req.OriginatorDepository, userId)
	if err != nil {
		return nil, nil, nil, nil, errors.New("Originator Depository not found")
	}
	if origDep.Status != DepositoryVerified {
		return nil, nil, nil, nil, fmt.Errorf("Originator Depository %s is in status %v", origDep.ID, origDep.Status)
	}
	if err := origDep.validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("originator depository: %v", err)
	}

	return cust, custDep, orig, origDep, nil
}

// createACHFile will take in a Transfer and metadata to build an ACH file.
// Returned is the ACH service File ID which can be used to retrieve the file (and it's contents).
func createACHFile(client *achclient.ACH, id, idempotencyKey, userId string, transfer *Transfer, cust *Customer, custDep *Depository, orig *Originator, origDep *Depository) (string, error) {
	if transfer.Type == PullTransfer && cust.Status != CustomerVerified {
		// TODO(adam): "additional checks" - check Customer.Status ???
		// https://github.com/moov-io/paygate/issues/18#issuecomment-432066045
		return "", fmt.Errorf("customer_id=%q is not Verified user_id=%q", cust.ID, userId)
	}
	if transfer.Status != TransferPending {
		return "", fmt.Errorf("transfer_id=%q is not Pending (status=%s)", transfer.ID, transfer.Status)
	}

	// Create our ACH file
	file, now := ach.NewFile(), time.Now()
	file.ID = id
	file.Control = ach.NewFileControl()

	// File Header
	file.Header.ID = id
	file.Header.ImmediateOrigin = origDep.RoutingNumber
	file.Header.ImmediateOriginName = origDep.BankName
	file.Header.ImmediateDestination = custDep.RoutingNumber
	file.Header.ImmediateDestinationName = custDep.BankName
	file.Header.FileCreationDate = now.Format("060102") // YYMMDD
	file.Header.FileCreationTime = now.Format("1504")   // HHMM

	// Add batch to our ACH file
	switch transfer.StandardEntryClassCode {
	case ach.IAT:
		batch, err := createIATBatch(id, userId, transfer, cust, custDep, orig, origDep)
		if err != nil {
			return "", err
		}
		file.IATBatches = append(file.IATBatches, *batch)
	case ach.PPD:
		batch, err := createPPDBatch(id, userId, transfer, cust, custDep, orig, origDep)
		if err != nil {
			return "", err
		}
		file.Batches = append(file.Batches, batch)
	case ach.WEB:
		batch, err := createWEBBatch(id, userId, transfer, cust, custDep, orig, origDep)
		if err != nil {
			return "", err
		}
		file.Batches = append(file.Batches, batch)
	default:
		return "", fmt.Errorf("unsupported SEC code: %s", transfer.StandardEntryClassCode)
	}

	// Create ACH File
	fileId, err := client.CreateFile(idempotencyKey, file)
	if err != nil {
		return "", fmt.Errorf("ACH File %s (userId=%s) failed to create file: %v", id, userId, err)
	}
	return fileId, nil
}

// checkACHFile calls out to our ACH service to build and validate the ACH file,
// "build" involves the ACH service computing some file/batch level totals and checksums.
func checkACHFile(client *achclient.ACH, fileId, userId string) error {
	// We don't care about the resposne, just the side-effect build tabulations.
	if _, err := client.GetFileContents(fileId); err != nil && logger != nil {
		logger.Log("transfers", fmt.Sprintf("userId=%s fileId=%s err=%v", userId, fileId, err))
	}
	// ValidateFile will return specific file-level information about what's wrong.
	return client.ValidateFile(fileId)
}

func determineServiceClassCode(t *Transfer) int {
	// Credits: 220, Debits: 225
	if t.Type == PushTransfer {
		return 220
	}
	return 225
}

func determineTransactionCode(t *Transfer) int {
	// Credit (deposit) to checking account ‘22’
	// Prenote for credit to checking account ‘23’
	// Debit (withdrawal) to checking account ‘27’
	// Prenote for debit to checking account ‘28’
	// Credit to savings account ‘32’
	// Prenote for credit to savings account ‘33’
	// Debit to savings account ‘37’
	// Prenote for debit to savings account ‘38’
	return 22 // TODO(adam): need to check input data
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

func writeTransferEvent(userId string, req *transferRequest, eventRepo eventRepository) error {
	return eventRepo.writeEvent(userId, &Event{
		ID:      EventID(nextID()),
		Topic:   fmt.Sprintf("%s transfer to %s", req.Type, req.Description),
		Message: req.Description,
		Type:    TransferEvent,
	})
}

func writeResponse(w http.ResponseWriter, reqCount int, transfers []*Transfer) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if reqCount == 1 {
		// don't render surrounding array for single transfer create
		// (it's coming from POST /transfers, not POST /transfers/batch)
		if err := json.NewEncoder(w).Encode(transfers[0]); err != nil {
			internalError(w, err)
			return
		}
	} else {
		if err := json.NewEncoder(w).Encode(transfers); err != nil {
			internalError(w, err)
			return
		}
	}
}

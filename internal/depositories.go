// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type DepositoryID string

func (id DepositoryID) empty() bool {
	return string(id) == ""
}

type Depository struct {
	// ID is a unique string representing this Depository.
	ID DepositoryID `json:"id"`

	// BankName is the legal name of the financial institution.
	BankName string `json:"bankName"`

	// Holder is the legal holder name on the account
	Holder string `json:"holder"`

	// HolderType defines the type of entity of the account holder as an individual or company
	HolderType HolderType `json:"holderType"`

	// Type defines the account as checking or savings
	Type AccountType `json:"type"`

	// RoutingNumber is the ABA routing transit number for the depository account.
	RoutingNumber string `json:"routingNumber"`

	// Status defines the current state of the Depository
	Status DepositoryStatus `json:"status"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`

	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created base.Time `json:"created"`

	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated base.Time `json:"updated"`

	// ReturnCodes holds the optional set of return codes for why this Depository was rejected
	ReturnCodes []*ach.ReturnCode `json:"returnCodes"`

	// non-exported fields
	userID string

	// EncryptedAccountNumber is the account number for the depository account encrypted
	// with the attached secrets.StringKeeper
	EncryptedAccountNumber string `json:"-"`
	hashedAccountNumber    string
	keeper                 *secrets.StringKeeper
}

func (d *Depository) UserID() string {
	if d == nil {
		return ""
	}
	return d.userID
}

func (d *Depository) validate() error {
	if d == nil {
		return errors.New("nil Depository")
	}
	if err := d.HolderType.validate(); err != nil {
		return err
	}
	if err := d.Type.Validate(); err != nil {
		return err
	}
	if err := d.Status.validate(); err != nil {
		return err
	}
	if err := ach.CheckRoutingNumber(d.RoutingNumber); err != nil {
		return err
	}
	if d.EncryptedAccountNumber == "" {
		return errors.New("missing Depository.EncryptedAccountNumber")
	}
	return nil
}

func (d *Depository) ReplaceAccountNumber(num string) error {
	if d == nil || d.keeper == nil {
		return errors.New("nil Depository and/or keeper")
	}
	encrypted, err := d.keeper.EncryptString(num)
	if err != nil {
		return err
	}
	hash, err := hashAccountNumber(num)
	if err != nil {
		return err
	}
	d.EncryptedAccountNumber = encrypted
	d.hashedAccountNumber = hash
	return nil
}

func (d *Depository) DecryptAccountNumber() (string, error) {
	if d == nil || d.keeper == nil {
		return "", errors.New("nil Depository or keeper")
	}
	num, err := d.keeper.DecryptString(d.EncryptedAccountNumber)
	if err != nil {
		return "", err
	}
	return num, nil
}

func (d Depository) MarshalJSON() ([]byte, error) {
	num, err := d.DecryptAccountNumber()
	if err != nil {
		return nil, err
	}
	type Aux Depository
	return json.Marshal(struct {
		Aux
		AccountNumber string `json:"accountNumber"`
	}{
		(Aux)(d),
		num,
	})
}

type depositoryRequest struct {
	bankName      string
	holder        string
	holderType    HolderType
	accountType   AccountType
	routingNumber string
	accountNumber string
	metadata      string

	keeper              *secrets.StringKeeper
	hashedAccountNumber string
}

func (r depositoryRequest) missingFields() error {
	if r.bankName == "" {
		return errors.New("missing depositoryRequest.BankName")
	}
	if r.holder == "" {
		return errors.New("missing depositoryRequest.Holder")
	}
	if r.holderType == "" {
		return errors.New("missing depositoryRequest.HolderType")
	}
	if r.accountType == "" {
		return errors.New("missing depositoryRequest.Type")
	}
	if r.routingNumber == "" {
		return errors.New("missing depositoryRequest.RoutingNumber")
	}
	if r.accountNumber == "" {
		return errors.New("missing depositoryRequest.AccountNumber")
	}
	return nil
}

func (r *depositoryRequest) UnmarshalJSON(data []byte) error {
	var wrapper struct {
		BankName      string      `json:"bankName,omitempty"`
		Holder        string      `json:"holder,omitempty"`
		HolderType    HolderType  `json:"holderType,omitempty"`
		AccountType   AccountType `json:"type,omitempty"`
		RoutingNumber string      `json:"routingNumber,omitempty"`
		AccountNumber string      `json:"accountNumber,omitempty"`
		Metadata      string      `json:"metadata,omitempty"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	r.bankName = wrapper.BankName
	r.holder = wrapper.Holder
	r.holderType = wrapper.HolderType
	r.accountType = wrapper.AccountType
	r.routingNumber = wrapper.RoutingNumber
	r.metadata = wrapper.Metadata

	if wrapper.AccountNumber != "" {
		if num, err := r.keeper.EncryptString(wrapper.AccountNumber); err != nil {
			return err
		} else {
			r.accountNumber = num
		}
		if hash, err := hashAccountNumber(wrapper.AccountNumber); err != nil {
			return err
		} else {
			r.hashedAccountNumber = hash
		}
	}

	return nil
}

type HolderType string

const (
	Individual HolderType = "individual"
	Business   HolderType = "business"
)

func (t *HolderType) empty() bool {
	return string(*t) == ""
}

func (t HolderType) validate() error {
	if t.empty() {
		return errors.New("empty HolderType")
	}
	switch t {
	case Individual, Business:
		return nil
	default:
		return fmt.Errorf("HolderType(%s) is invalid", t)
	}
}

func (t *HolderType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = HolderType(strings.ToLower(s))
	if err := t.validate(); err != nil {
		return err
	}
	return nil
}

type DepositoryStatus string

const (
	DepositoryUnverified DepositoryStatus = "unverified"
	DepositoryVerified   DepositoryStatus = "verified"
	DepositoryRejected   DepositoryStatus = "rejected"
)

func (ds DepositoryStatus) empty() bool {
	return string(ds) == ""
}

func (ds DepositoryStatus) validate() error {
	switch ds {
	case DepositoryUnverified, DepositoryVerified, DepositoryRejected:
		return nil
	default:
		return fmt.Errorf("DepositoryStatus(%s) is invalid", ds)
	}
}

func (ds *DepositoryStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*ds = DepositoryStatus(strings.ToLower(s))
	if err := ds.validate(); err != nil {
		return err
	}
	return nil
}

type DepositoryRouter struct {
	logger log.Logger

	odfiAccount *ODFIAccount

	achClient      *achclient.ACH
	accountsClient AccountsClient
	fedClient      fed.Client

	microDepositAttemper attempter

	depositoryRepo DepositoryRepository
	eventRepo      EventRepository

	keeper *secrets.StringKeeper
}

func NewDepositoryRouter(
	logger log.Logger,
	odfiAccount *ODFIAccount,
	accountsClient AccountsClient,
	achClient *achclient.ACH,
	fedClient fed.Client,
	depositoryRepo DepositoryRepository,
	eventRepo EventRepository,
	keeper *secrets.StringKeeper,
) *DepositoryRouter {

	router := &DepositoryRouter{
		logger:         logger,
		odfiAccount:    odfiAccount,
		achClient:      achClient,
		accountsClient: accountsClient,
		fedClient:      fedClient,
		depositoryRepo: depositoryRepo,
		eventRepo:      eventRepo,
		keeper:         keeper,
	}
	if r, ok := depositoryRepo.(*SQLDepositoryRepo); ok {
		// only allow 5 micro-deposit verification steps
		router.microDepositAttemper = NewAttemper(logger, r.db, 5)
	}
	return router
}

func (r *DepositoryRouter) RegisterRoutes(router *mux.Router) {
	router.Methods("GET").Path("/depositories").HandlerFunc(r.getUserDepositories())
	router.Methods("POST").Path("/depositories").HandlerFunc(r.createUserDepository())

	router.Methods("GET").Path("/depositories/{depositoryId}").HandlerFunc(r.getUserDepository())
	router.Methods("PATCH").Path("/depositories/{depositoryId}").HandlerFunc(r.updateUserDepository())
	router.Methods("DELETE").Path("/depositories/{depositoryId}").HandlerFunc(r.deleteUserDepository())

	router.Methods("POST").Path("/depositories/{depositoryId}/micro-deposits").HandlerFunc(r.initiateMicroDeposits())
	router.Methods("POST").Path("/depositories/{depositoryId}/micro-deposits/confirm").HandlerFunc(r.confirmMicroDeposits())
}

// GET /depositories
// response: [ depository ]
func (r *DepositoryRouter) getUserDepositories() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		w, err := wrapResponseWriter(r.logger, w, httpReq)
		if err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		userID := moovhttp.GetUserID(httpReq)
		deposits, err := r.depositoryRepo.GetUserDepositories(userID)
		if err != nil {
			r.logger.Log("depositories", fmt.Sprintf("problem reading user depositories"), "requestID", moovhttp.GetRequestID(httpReq), "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		for i := range deposits {
			deposits[i].keeper = r.keeper
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(deposits)
	}
}

func readDepositoryRequest(r *http.Request, keeper *secrets.StringKeeper) (depositoryRequest, error) {
	req := depositoryRequest{
		keeper: keeper,
	}
	bs, err := read(r.Body)
	if err != nil {
		return req, err
	}
	if err := json.Unmarshal(bs, &req); err != nil {
		return req, err
	}
	if req.accountNumber != "" {
		if num, err := keeper.DecryptString(req.accountNumber); err != nil {
			return req, err
		} else {
			if hash, err := hashAccountNumber(num); err != nil {
				return req, err
			} else {
				req.hashedAccountNumber = hash
			}
		}
	}
	return req, nil
}

// POST /depositories
// request: model w/o ID
// response: 201 w/ depository json
func (r *DepositoryRouter) createUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		w, err := wrapResponseWriter(r.logger, w, httpReq)
		if err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		requestID, userID := moovhttp.GetRequestID(httpReq), moovhttp.GetUserID(httpReq)

		req, err := readDepositoryRequest(httpReq, r.keeper)
		if err != nil {
			r.logger.Log("depositories", err, "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		if err := req.missingFields(); err != nil {
			err = fmt.Errorf("%v: %v", errMissingRequiredJson, err)
			moovhttp.Problem(w, err)
			return
		}

		now := time.Now()
		depository := &Depository{
			ID:                     DepositoryID(base.ID()),
			BankName:               req.bankName,
			Holder:                 req.holder,
			HolderType:             req.holderType,
			Type:                   req.accountType,
			RoutingNumber:          req.routingNumber,
			Status:                 DepositoryUnverified,
			Metadata:               req.metadata,
			Created:                base.NewTime(now),
			Updated:                base.NewTime(now),
			userID:                 userID,
			keeper:                 r.keeper,
			EncryptedAccountNumber: req.accountNumber,
			hashedAccountNumber:    req.hashedAccountNumber,
		}
		if err := depository.validate(); err != nil {
			r.logger.Log("depositories", err.Error(), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		// TODO(adam): We should check and reject duplicate Depositories (by ABA and AccountNumber) on creation

		// Check FED for the routing number
		if err := r.fedClient.LookupRoutingNumber(req.routingNumber); err != nil {
			r.logger.Log("depositories", fmt.Sprintf("problem with FED routing number lookup %q: %v", req.routingNumber, err.Error()), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		if err := r.depositoryRepo.UpsertUserDepository(userID, depository); err != nil {
			r.logger.Log("depositories", err.Error(), "requestID", requestID, "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(depository)
	}
}

func (r *DepositoryRouter) getUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		w, err := wrapResponseWriter(r.logger, w, httpReq)
		if err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		id, userID := GetDepositoryID(httpReq), moovhttp.GetUserID(httpReq)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		depository, err := r.depositoryRepo.GetUserDepository(id, userID)
		if err != nil {
			r.logger.Log("depositories", err.Error(), "requestID", moovhttp.GetRequestID(httpReq), "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		depository.keeper = r.keeper

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(depository)
	}
}

func (r *DepositoryRouter) updateUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		w, err := wrapResponseWriter(r.logger, w, httpReq)
		if err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		req, err := readDepositoryRequest(httpReq, r.keeper)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		id, userID := GetDepositoryID(httpReq), moovhttp.GetUserID(httpReq)
		if id == "" || userID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		depository, err := r.depositoryRepo.GetUserDepository(id, userID)
		if err != nil {
			r.logger.Log("depositories", err.Error(), "requestID", moovhttp.GetRequestID(httpReq), "userID", userID)
			moovhttp.Problem(w, err)
			return
		}
		if depository == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Update model
		var requireValidation bool
		if req.bankName != "" {
			depository.BankName = req.bankName
		}
		if req.holder != "" {
			depository.Holder = req.holder
		}
		if req.holderType != "" {
			depository.HolderType = req.holderType
		}
		if req.accountType != "" {
			depository.Type = req.accountType
		}
		if req.routingNumber != "" {
			if err := ach.CheckRoutingNumber(req.routingNumber); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			requireValidation = true
			depository.RoutingNumber = req.routingNumber
		}
		if req.accountNumber != "" {
			requireValidation = true
			// readDepositoryRequest encrypts and hashes for us
			depository.EncryptedAccountNumber = req.accountNumber
			depository.hashedAccountNumber = req.hashedAccountNumber
		}
		if req.metadata != "" {
			depository.Metadata = req.metadata
		}
		depository.Updated = base.NewTime(time.Now())

		if requireValidation {
			depository.Status = DepositoryUnverified
		}

		if err := depository.validate(); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		if err := r.depositoryRepo.UpsertUserDepository(userID, depository); err != nil {
			r.logger.Log("depositories", err.Error(), "requestID", moovhttp.GetRequestID(httpReq), "userID", userID)
			moovhttp.Problem(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(depository)
	}
}

func (r *DepositoryRouter) deleteUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		w, err := wrapResponseWriter(r.logger, w, httpReq)
		if err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		id, userID := GetDepositoryID(httpReq), moovhttp.GetUserID(httpReq)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := r.depositoryRepo.deleteUserDepository(id, userID); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// GetDepositoryID extracts the DepositoryID from the incoming request.
func GetDepositoryID(r *http.Request) DepositoryID {
	v := mux.Vars(r)
	id, ok := v["depositoryId"]
	if !ok {
		return DepositoryID("")
	}
	return DepositoryID(id)
}

func markDepositoryVerified(repo DepositoryRepository, id DepositoryID, userID string) error {
	dep, err := repo.GetUserDepository(id, userID)
	if err != nil {
		return fmt.Errorf("markDepositoryVerified: depository %v (userID=%v): %v", id, userID, err)
	}
	dep.Status = DepositoryVerified
	return repo.UpsertUserDepository(userID, dep)
}

type DepositoryRepository interface {
	GetDepository(id DepositoryID) (*Depository, error) // admin endpoint
	GetUserDepositories(userID string) ([]*Depository, error)
	GetUserDepository(id DepositoryID, userID string) (*Depository, error)

	UpsertUserDepository(userID string, dep *Depository) error
	UpdateDepositoryStatus(id DepositoryID, status DepositoryStatus) error
	deleteUserDepository(id DepositoryID, userID string) error

	GetMicroDeposits(id DepositoryID) ([]*MicroDeposit, error) // admin endpoint
	getMicroDepositsForUser(id DepositoryID, userID string) ([]*MicroDeposit, error)

	LookupDepositoryFromReturn(routingNumber string, accountNumber string) (*Depository, error)
	LookupMicroDepositFromReturn(id DepositoryID, amount *Amount) (*MicroDeposit, error)
	SetReturnCode(id DepositoryID, amount Amount, returnCode string) error

	InitiateMicroDeposits(id DepositoryID, userID string, microDeposit []*MicroDeposit) error
	confirmMicroDeposits(id DepositoryID, userID string, amounts []Amount) error
	GetMicroDepositCursor(batchSize int) *MicroDepositCursor
}

func NewDepositoryRepo(logger log.Logger, db *sql.DB, keeper *secrets.StringKeeper) *SQLDepositoryRepo {
	return &SQLDepositoryRepo{logger: logger, db: db, keeper: keeper}
}

type SQLDepositoryRepo struct {
	db     *sql.DB
	logger log.Logger
	keeper *secrets.StringKeeper
}

func (r *SQLDepositoryRepo) Close() error {
	return r.db.Close()
}

func (r *SQLDepositoryRepo) GetDepository(id DepositoryID) (*Depository, error) {
	query := `select user_id from depositories where depository_id = ? and deleted_at is null limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var userID string
	if err := stmt.QueryRow(id).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if userID == "" {
		return nil, nil // not found
	}

	dep, err := r.GetUserDepository(id, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return dep, err
}

func (r *SQLDepositoryRepo) GetUserDepositories(userID string) ([]*Depository, error) {
	query := `select depository_id from depositories where user_id = ? and deleted_at is null`
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

	var depositoryIds []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return nil, fmt.Errorf("GetUserDepositories scan: %v", err)
		}
		if row != "" {
			depositoryIds = append(depositoryIds, row)
		}
	}

	var depositories []*Depository
	for i := range depositoryIds {
		dep, err := r.GetUserDepository(DepositoryID(depositoryIds[i]), userID)
		if err == nil && dep != nil && dep.BankName != "" {
			depositories = append(depositories, dep)
		}
	}
	return depositories, rows.Err()
}

func (r *SQLDepositoryRepo) GetUserDepository(id DepositoryID, userID string) (*Depository, error) {
	query := `select depository_id, bank_name, holder, holder_type, type, routing_number, account_number_encrypted, account_number_hashed, status, metadata, created_at, last_updated_at
from depositories
where depository_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("GetUserDepository: prepare: %v", err)
	}
	defer stmt.Close()

	row := stmt.QueryRow(id, userID)

	dep := &Depository{userID: userID}
	var (
		created time.Time
		updated time.Time
	)
	err = row.Scan(&dep.ID, &dep.BankName, &dep.Holder, &dep.HolderType, &dep.Type, &dep.RoutingNumber, &dep.EncryptedAccountNumber, &dep.hashedAccountNumber, &dep.Status, &dep.Metadata, &created, &updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("GetUserDepository: scan: %v", err)
	}
	dep.ReturnCodes = r.getMicroDepositReturnCodes(dep.ID)
	dep.Created = base.NewTime(created)
	dep.Updated = base.NewTime(updated)
	if dep.ID == "" || dep.BankName == "" {
		return nil, nil // no records found
	}
	dep.keeper = r.keeper
	return dep, nil
}

func (r *SQLDepositoryRepo) UpsertUserDepository(userID string, dep *Depository) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	now := base.NewTime(time.Now())
	if dep.Created.IsZero() {
		dep.Created = now
		dep.Updated = now
	}

	query := `insert into depositories (depository_id, user_id, bank_name, holder, holder_type, type, routing_number, account_number_encrypted, account_number_hashed, status, metadata, created_at, last_updated_at)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}

	res, err := stmt.Exec(dep.ID, userID, dep.BankName, dep.Holder, dep.HolderType, dep.Type, dep.RoutingNumber, dep.EncryptedAccountNumber, dep.hashedAccountNumber, dep.Status, dep.Metadata, dep.Created.Time, dep.Updated.Time)
	stmt.Close()
	if err != nil && !database.UniqueViolation(err) {
		return fmt.Errorf("problem upserting depository=%q, userID=%q: %v", dep.ID, userID, err)
	}
	if res != nil {
		if n, _ := res.RowsAffected(); n != 0 {
			return tx.Commit() // Depository was inserted, so cleanup and exit
		}
	}
	query = `update depositories
set bank_name = ?, holder = ?, holder_type = ?, type = ?, routing_number = ?,
account_number_encrypted = ?, account_number_hashed = ?, status = ?, metadata = ?, last_updated_at = ?
where depository_id = ? and user_id = ? and deleted_at is null`
	stmt, err = tx.Prepare(query)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(
		dep.BankName, dep.Holder, dep.HolderType, dep.Type, dep.RoutingNumber,
		dep.EncryptedAccountNumber, dep.hashedAccountNumber, dep.Status, dep.Metadata, time.Now(), dep.ID, userID)
	stmt.Close()
	if err != nil {
		return fmt.Errorf("UpsertUserDepository: exec error=%v rollback=%v", err, tx.Rollback())
	}
	return tx.Commit()
}

func (r *SQLDepositoryRepo) UpdateDepositoryStatus(id DepositoryID, status DepositoryStatus) error {
	query := `update depositories set status = ?, last_updated_at = ? where depository_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(status, time.Now(), id); err != nil {
		return fmt.Errorf("error updating status depository_id=%q: %v", id, err)
	}
	return nil
}

func (r *SQLDepositoryRepo) deleteUserDepository(id DepositoryID, userID string) error {
	query := `update depositories set deleted_at = ? where depository_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(time.Now(), id, userID); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error deleting depository_id=%q, user_id=%q: %v", id, userID, err)
	}
	return nil
}

func (r *SQLDepositoryRepo) LookupDepositoryFromReturn(routingNumber string, accountNumber string) (*Depository, error) {
	hash, err := hashAccountNumber(accountNumber)
	if err != nil {
		return nil, err
	}
	// order by created_at to ignore older rows with non-null deleted_at's
	query := `select depository_id, user_id from depositories where routing_number = ? and account_number_hashed = ? and deleted_at is null order by created_at desc limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	depID, userID := "", ""
	if err := stmt.QueryRow(routingNumber, hash).Scan(&depID, &userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("LookupDepositoryFromReturn: %v", err)
	}
	return r.GetUserDepository(DepositoryID(depID), userID)
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/hash"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/achclient"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type depositoryRequest struct {
	bankName      string
	holder        string
	holderType    model.HolderType
	accountType   model.AccountType
	routingNumber string
	accountNumber string
	metadata      string

	keeper              *secrets.StringKeeper
	HashedAccountNumber string
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
		BankName      string            `json:"bankName,omitempty"`
		Holder        string            `json:"holder,omitempty"`
		HolderType    model.HolderType  `json:"holderType,omitempty"`
		AccountType   model.AccountType `json:"type,omitempty"`
		RoutingNumber string            `json:"routingNumber,omitempty"`
		AccountNumber string            `json:"accountNumber,omitempty"`
		Metadata      string            `json:"metadata,omitempty"`
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
		if hash, err := hash.AccountNumber(wrapper.AccountNumber); err != nil {
			return err
		} else {
			r.HashedAccountNumber = hash
		}
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
	eventRepo      events.Repository

	keeper *secrets.StringKeeper
}

func NewDepositoryRouter(
	logger log.Logger,
	odfiAccount *ODFIAccount,
	accountsClient AccountsClient,
	achClient *achclient.ACH,
	fedClient fed.Client,
	depositoryRepo DepositoryRepository,
	eventRepo events.Repository,
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
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		deposits, err := r.depositoryRepo.GetUserDepositories(responder.XUserID)
		if err != nil {
			responder.Log("depositories", fmt.Sprintf("problem reading user depositories"))
			responder.Problem(err)
			return
		}
		for i := range deposits {
			deposits[i].Keeper = r.keeper
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(deposits)
		})
	}
}

func readDepositoryRequest(r *http.Request, keeper *secrets.StringKeeper) (depositoryRequest, error) {
	wrapper := depositoryRequest{
		keeper: keeper,
	}
	if err := json.NewDecoder(Read(r.Body)).Decode(&wrapper); err != nil {
		return wrapper, err
	}
	if wrapper.accountNumber != "" {
		if num, err := keeper.DecryptString(wrapper.accountNumber); err != nil {
			return wrapper, err
		} else {
			if hash, err := hash.AccountNumber(num); err != nil {
				return wrapper, err
			} else {
				wrapper.HashedAccountNumber = hash
			}
		}
	}
	return wrapper, nil
}

// POST /depositories
// request: model w/o ID
// response: 201 w/ depository json
func (r *DepositoryRouter) createUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		req, err := readDepositoryRequest(httpReq, r.keeper)
		if err != nil {
			responder.Log("depositories", err, "requestID")
			responder.Problem(err)
			return
		}
		if err := req.missingFields(); err != nil {
			err = fmt.Errorf("%v: %v", ErrMissingRequiredJson, err)
			responder.Problem(err)
			return
		}

		now := time.Now()
		depository := &model.Depository{
			ID:                     id.Depository(base.ID()),
			BankName:               req.bankName,
			Holder:                 req.holder,
			HolderType:             req.holderType,
			Type:                   req.accountType,
			RoutingNumber:          req.routingNumber,
			Status:                 model.DepositoryUnverified,
			Metadata:               req.metadata,
			Created:                base.NewTime(now),
			Updated:                base.NewTime(now),
			UserID:                 responder.XUserID,
			Keeper:                 r.keeper,
			EncryptedAccountNumber: req.accountNumber,
			HashedAccountNumber:    req.HashedAccountNumber,
		}
		if err := depository.Validate(); err != nil {
			responder.Log("depositories", err.Error())
			responder.Problem(err)
			return
		}

		// TODO(adam): We should check and reject duplicate Depositories (by ABA and AccountNumber) on creation

		// Check FED for the routing number
		if err := r.fedClient.LookupRoutingNumber(req.routingNumber); err != nil {
			responder.Log("depositories", fmt.Sprintf("problem with FED routing number lookup %q: %v", req.routingNumber, err.Error()))
			responder.Problem(err)
			return
		}

		if err := r.depositoryRepo.UpsertUserDepository(responder.XUserID, depository); err != nil {
			responder.Log("depositories", err.Error())
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(depository)
		})
	}
}

func (r *DepositoryRouter) getUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		depID := GetDepositoryID(httpReq)
		if depID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		depository, err := r.depositoryRepo.GetUserDepository(depID, responder.XUserID)
		if err != nil {
			responder.Log("depositories", err.Error())
			responder.Problem(err)
			return
		}
		depository.Keeper = r.keeper

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(depository)
		})
	}
}

func (r *DepositoryRouter) updateUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		req, err := readDepositoryRequest(httpReq, r.keeper)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		depID := GetDepositoryID(httpReq)
		if depID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		depository, err := r.depositoryRepo.GetUserDepository(depID, responder.XUserID)
		if err != nil {
			r.logger.Log("depositories", err.Error())
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
				responder.Problem(err)
				return
			}
			requireValidation = true
			depository.RoutingNumber = req.routingNumber
		}
		if req.accountNumber != "" {
			requireValidation = true
			// readDepositoryRequest encrypts and hashes for us
			depository.EncryptedAccountNumber = req.accountNumber
			depository.HashedAccountNumber = req.HashedAccountNumber
		}
		if req.metadata != "" {
			depository.Metadata = req.metadata
		}
		depository.Updated = base.NewTime(time.Now())

		if requireValidation {
			depository.Status = model.DepositoryUnverified
		}

		if err := depository.Validate(); err != nil {
			responder.Problem(err)
			return
		}

		if err := r.depositoryRepo.UpsertUserDepository(responder.XUserID, depository); err != nil {
			responder.Log("depositories", err.Error())
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(depository)
		})
	}
}

func (r *DepositoryRouter) deleteUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		depID := GetDepositoryID(httpReq)
		if depID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := r.depositoryRepo.deleteUserDepository(depID, responder.XUserID); err != nil {
			moovhttp.Problem(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// GetDepositoryID extracts the id.Depository from the incoming request.
func GetDepositoryID(r *http.Request) id.Depository {
	v, ok := mux.Vars(r)["depositoryId"]
	if !ok {
		return id.Depository("")
	}
	return id.Depository(v)
}

func markDepositoryVerified(repo DepositoryRepository, depID id.Depository, userID id.User) error {
	dep, err := repo.GetUserDepository(depID, userID)
	if err != nil {
		return fmt.Errorf("markDepositoryVerified: depository %v (userID=%v): %v", depID, userID, err)
	}
	dep.Status = model.DepositoryVerified
	return repo.UpsertUserDepository(userID, dep)
}

type DepositoryRepository interface {
	GetDepository(id id.Depository) (*model.Depository, error) // admin endpoint
	GetUserDepositories(userID id.User) ([]*model.Depository, error)
	GetUserDepository(id id.Depository, userID id.User) (*model.Depository, error)

	UpsertUserDepository(userID id.User, dep *model.Depository) error
	UpdateDepositoryStatus(id id.Depository, status model.DepositoryStatus) error
	deleteUserDepository(id id.Depository, userID id.User) error

	GetMicroDeposits(id id.Depository) ([]*MicroDeposit, error) // admin endpoint
	getMicroDepositsForUser(id id.Depository, userID id.User) ([]*MicroDeposit, error)

	LookupDepositoryFromReturn(routingNumber string, accountNumber string) (*model.Depository, error)
	LookupMicroDepositFromReturn(id id.Depository, amount *model.Amount) (*MicroDeposit, error)
	SetReturnCode(id id.Depository, amount model.Amount, returnCode string) error

	InitiateMicroDeposits(id id.Depository, userID id.User, microDeposit []*MicroDeposit) error
	confirmMicroDeposits(id id.Depository, userID id.User, amounts []model.Amount) error
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

func (r *SQLDepositoryRepo) GetDepository(depID id.Depository) (*model.Depository, error) {
	query := `select user_id from depositories where depository_id = ? and deleted_at is null limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var userID string
	if err := stmt.QueryRow(depID).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if userID == "" {
		return nil, nil // not found
	}

	dep, err := r.GetUserDepository(depID, id.User(userID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return dep, err
}

func (r *SQLDepositoryRepo) GetUserDepositories(userID id.User) ([]*model.Depository, error) {
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

	var depositories []*model.Depository
	for i := range depositoryIds {
		dep, err := r.GetUserDepository(id.Depository(depositoryIds[i]), userID)
		if err == nil && dep != nil && dep.BankName != "" {
			depositories = append(depositories, dep)
		}
	}
	return depositories, rows.Err()
}

func (r *SQLDepositoryRepo) GetUserDepository(id id.Depository, userID id.User) (*model.Depository, error) {
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

	dep := &model.Depository{UserID: userID}
	var (
		created time.Time
		updated time.Time
	)
	err = row.Scan(&dep.ID, &dep.BankName, &dep.Holder, &dep.HolderType, &dep.Type, &dep.RoutingNumber, &dep.EncryptedAccountNumber, &dep.HashedAccountNumber, &dep.Status, &dep.Metadata, &created, &updated)
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
	dep.Keeper = r.keeper
	return dep, nil
}

func (r *SQLDepositoryRepo) UpsertUserDepository(userID id.User, dep *model.Depository) error {
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
	defer stmt.Close()

	res, err := stmt.Exec(dep.ID, userID, dep.BankName, dep.Holder, dep.HolderType, dep.Type, dep.RoutingNumber, dep.EncryptedAccountNumber, dep.HashedAccountNumber, dep.Status, dep.Metadata, dep.Created.Time, dep.Updated.Time)
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
	defer stmt.Close()
	_, err = stmt.Exec(
		dep.BankName, dep.Holder, dep.HolderType, dep.Type, dep.RoutingNumber,
		dep.EncryptedAccountNumber, dep.HashedAccountNumber, dep.Status, dep.Metadata, time.Now(), dep.ID, userID)
	stmt.Close()
	if err != nil {
		return fmt.Errorf("UpsertUserDepository: exec error=%v rollback=%v", err, tx.Rollback())
	}
	return tx.Commit()
}

func (r *SQLDepositoryRepo) UpdateDepositoryStatus(id id.Depository, status model.DepositoryStatus) error {
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

func (r *SQLDepositoryRepo) deleteUserDepository(id id.Depository, userID id.User) error {
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

func (r *SQLDepositoryRepo) LookupDepositoryFromReturn(routingNumber string, accountNumber string) (*model.Depository, error) {
	hash, err := hash.AccountNumber(accountNumber)
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
	return r.GetUserDepository(id.Depository(depID), id.User(userID))
}

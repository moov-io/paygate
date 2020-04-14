// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/hash"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/internal/secrets"
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

type Router struct {
	logger log.Logger

	fedClient fed.Client

	depositoryRepo Repository
	eventRepo      events.Repository

	keeper *secrets.StringKeeper
}

func NewRouter(
	logger log.Logger,
	fedClient fed.Client,
	depositoryRepo Repository,
	eventRepo events.Repository,
	keeper *secrets.StringKeeper,
) *Router {
	router := &Router{
		logger:         logger,
		fedClient:      fedClient,
		depositoryRepo: depositoryRepo,
		eventRepo:      eventRepo,
		keeper:         keeper,
	}
	return router
}

func (r *Router) RegisterRoutes(router *mux.Router) {
	router.Methods("GET").Path("/depositories").HandlerFunc(r.getUserDepositories())
	router.Methods("POST").Path("/depositories").HandlerFunc(r.createUserDepository())

	router.Methods("GET").Path("/depositories/{depositoryId}").HandlerFunc(r.getUserDepository())
	router.Methods("PATCH").Path("/depositories/{depositoryId}").HandlerFunc(r.updateUserDepository())
	router.Methods("DELETE").Path("/depositories/{depositoryId}").HandlerFunc(r.deleteUserDepository())
}

// GET /depositories
// response: [ depository ]
func (r *Router) getUserDepositories() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		deposits, err := r.depositoryRepo.getUserDepositories(responder.XUserID)
		if err != nil {
			responder.Log("depositories", "problem reading user depositories")
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
	if err := json.NewDecoder(route.Read(r.Body)).Decode(&wrapper); err != nil {
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
func (r *Router) createUserDepository() http.HandlerFunc {
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
			err = fmt.Errorf("%v: %v", route.ErrMissingRequiredJson, err)
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

		// Check FED for the routing number
		if r.fedClient != nil {
			if err := r.fedClient.LookupRoutingNumber(req.routingNumber); err != nil {
				err = fmt.Errorf("problem with FED routing number lookup %q: %v", req.routingNumber, err)
				responder.Log("depositories", err)
				responder.Problem(err)
				return
			}
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

func (r *Router) getUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		depID := GetID(httpReq)
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
		if depository != nil {
			depository.Keeper = r.keeper
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(depository)
		})
	}
}

func (r *Router) updateUserDepository() http.HandlerFunc {
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

		depID := GetID(httpReq)
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

type RemoveMicroDeposits struct {
	DepositoryID id.Depository

	XRequestID string
	XUserID    id.User

	Waiter chan interface{}
}

func (req *RemoveMicroDeposits) send(controller chan interface{}) {
	req.Waiter = make(chan interface{}, 1)
	if controller != nil {
		controller <- req
		<-req.Waiter
	}
}

func (r *Router) deleteUserDepository() http.HandlerFunc {
	return func(w http.ResponseWriter, httpReq *http.Request) {
		responder := route.NewResponder(r.logger, w, httpReq)
		if responder == nil {
			return
		}

		depID := GetID(httpReq)
		if depID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Currently we don't delete any pending Transfers associated to this Depository.
		// This could be done, but isn't as we're relying on the caller to delete Transfers they don't
		// want sent off to the ODFI.

		if err := r.depositoryRepo.deleteUserDepository(depID, responder.XUserID); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// GetID extracts the id.Depository from the incoming request.
func GetID(r *http.Request) id.Depository {
	v, ok := mux.Vars(r)["depositoryId"]
	if !ok {
		return id.Depository("")
	}
	return id.Depository(v)
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/base/idempotent"
	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/gateways"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/originators"
	"github.com/moov-io/paygate/internal/receivers"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/internal/util"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type transferRequest struct {
	Type                   model.TransferType `json:"transferType"`
	Amount                 model.Amount       `json:"amount"`
	Originator             model.OriginatorID `json:"originator"`
	OriginatorDepository   id.Depository      `json:"originatorDepository"`
	Receiver               model.ReceiverID   `json:"receiver"`
	ReceiverDepository     id.Depository      `json:"receiverDepository"`
	Description            string             `json:"description,omitempty"`
	StandardEntryClassCode string             `json:"standardEntryClassCode"`
	SameDay                bool               `json:"sameDay,omitempty"`

	CCDDetail *model.CCDDetail `json:"CCDDetail,omitempty"`
	IATDetail *model.IATDetail `json:"IATDetail,omitempty"`
	PPDDetail *model.PPDDetail `json:"PPDDetail,omitempty"`
	TELDetail *model.TELDetail `json:"TELDetail,omitempty"`
	WEBDetail *model.WEBDetail `json:"WEBDetail,omitempty"`

	// Internal fields for auditing and tracing
	fileID          string
	transactionID   string
	remoteAddr      string
	userID          id.User
	overAmountLimit bool
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

func (r transferRequest) asTransfer(transferID string) *model.Transfer {
	xfer := &model.Transfer{
		ID:                     id.Transfer(transferID),
		Type:                   r.Type,
		Amount:                 r.Amount,
		Originator:             r.Originator,
		OriginatorDepository:   r.OriginatorDepository,
		Receiver:               r.Receiver,
		ReceiverDepository:     r.ReceiverDepository,
		Description:            r.Description,
		StandardEntryClassCode: r.StandardEntryClassCode,
		Status:                 model.TransferPending,
		SameDay:                r.SameDay,
		Created:                base.Now(),
		UserID:                 r.userID.String(),
	}
	if r.overAmountLimit {
		xfer.Status = model.TransferReviewable
	}
	// Copy along the YYYDetail sub-object for specific SEC codes
	// where we expect one in the JSON request body.
	switch xfer.StandardEntryClassCode {
	case ach.CCD:
		xfer.CCDDetail = r.CCDDetail
	case ach.IAT:
		xfer.IATDetail = r.IATDetail
	case ach.PPD:
		xfer.PPDDetail = r.PPDDetail
	case ach.TEL:
		xfer.TELDetail = r.TELDetail
	case ach.WEB:
		xfer.WEBDetail = r.WEBDetail
	}
	return xfer
}

type TransferRouter struct {
	logger log.Logger

	depRepo            depository.Repository
	eventRepo          events.Repository
	gatewayRepo        gateways.Repository
	receiverRepository receivers.Repository
	origRepo           originators.Repository
	transferRepo       Repository

	transferLimitChecker *LimitChecker

	accountsClient  accounts.Client
	customersClient customers.Client
}

func NewTransferRouter(
	logger log.Logger,
	depositoryRepo depository.Repository,
	eventRepo events.Repository,
	gatewayRepo gateways.Repository,
	receiverRepo receivers.Repository,
	originatorsRepo originators.Repository,
	transferRepo Repository,
	transferLimitChecker *LimitChecker,
	accountsClient accounts.Client,
	customersClient customers.Client,
) *TransferRouter {
	return &TransferRouter{
		logger:               logger,
		depRepo:              depositoryRepo,
		eventRepo:            eventRepo,
		gatewayRepo:          gatewayRepo,
		receiverRepository:   receiverRepo,
		origRepo:             originatorsRepo,
		transferRepo:         transferRepo,
		transferLimitChecker: transferLimitChecker,
		accountsClient:       accountsClient,
		customersClient:      customersClient,
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

func getTransferID(r *http.Request) id.Transfer {
	vars := mux.Vars(r)
	v, ok := vars["transferId"]
	if ok {
		return id.Transfer(v)
	}
	return id.Transfer("")
}

type transferFilterParams struct {
	Status    model.TransferStatus
	StartDate time.Time
	EndDate   time.Time
	Limit     int64
	Offset    int64
}

func readTransferFilterParams(r *http.Request) transferFilterParams {
	params := transferFilterParams{
		StartDate: time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Now().Add(24 * time.Hour),
		Limit:     100,
		Offset:    0,
	}
	if r == nil {
		return params
	}
	q := r.URL.Query()
	if v := q.Get("startDate"); v != "" {
		params.StartDate = util.FirstParsedTime(v, base.ISO8601Format, util.YYMMDDTimeFormat)
	}
	if v := q.Get("endDate"); v != "" {
		params.EndDate, _ = time.Parse(base.ISO8601Format, v)
	}
	if status := model.TransferStatus(q.Get("status")); status.Validate() == nil {
		params.Status = status
	}
	if limit := route.ReadLimit(r); limit != 0 {
		params.Limit = limit
	}
	if offset := route.ReadOffset(r); offset != 0 {
		params.Offset = offset
	}
	return params
}

func (c *TransferRouter) getUserTransfers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		if responder == nil {
			return
		}

		params := readTransferFilterParams(r)
		transfers, err := c.transferRepo.getUserTransfers(responder.XUserID, params)
		if err != nil {
			responder.Log("transfers", fmt.Sprintf("error getting user transfers: %v", err))
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(transfers)
		})
	}
}

func (c *TransferRouter) getUserTransfer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		if responder == nil {
			return
		}

		transferID := getTransferID(r)
		transfer, err := c.transferRepo.getUserTransfer(transferID, responder.XUserID)
		if err != nil {
			responder.Log("transfers", fmt.Sprintf("error reading transfer=%s: %v", transferID, err))
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(transfer)
		})
	}
}

// readTransferRequests will attempt to parse the incoming body as either a transferRequest or []transferRequest.
// If no requests were read a non-nil error is returned.
func readTransferRequests(r *http.Request) ([]*transferRequest, error) {
	bs, err := ioutil.ReadAll(route.Read(r.Body))
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
		responder := route.NewResponder(c.logger, w, r)
		if responder == nil {
			return
		}

		requests, err := readTransferRequests(r)
		if err != nil {
			responder.Problem(err)
			return
		}

		// Carry over any incoming idempotency key and set one otherwise
		idempotencyKey := idempotent.Header(r)
		if idempotencyKey == "" {
			idempotencyKey = base.ID()
		}
		remoteIP := route.RemoteAddr(r.Header)

		gateway, err := c.gatewayRepo.GetUserGateway(responder.XUserID)
		if gateway == nil || err != nil {
			responder.Problem(fmt.Errorf("missing Gateway: %v", err))
			return
		}

		for i := range requests {
			transferID, req := base.ID(), requests[i]
			if err := req.missingFields(); err != nil {
				responder.Problem(err)
				return
			}
			req.remoteAddr = remoteIP
			req.userID = responder.XUserID

			// Grab and validate objects required for this transfer.
			receiver, receiverDep, orig, origDep, err := c.getTransferObjects(responder.XUserID, req.Originator, req.OriginatorDepository, req.Receiver, req.ReceiverDepository)
			if err != nil {
				objects := fmt.Sprintf("receiver=%v, receiverDep=%v, orig=%v, origDep=%v, err: %v", receiver, receiverDep, orig, origDep, err)
				responder.Log("transfers", fmt.Sprintf("Unable to find all objects during transfer create for user_id=%s, %s", responder.XUserID, objects))

				// Respond back to user
				responder.Problem(fmt.Errorf("missing data to create transfer: %s", err))
				return
			}

			// Check limits for this userID and destination
			// TODO(adam): We'll need user level limit overrides
			if err := c.transferLimitChecker.allowTransfer(responder.XUserID); err != nil {
				if strings.Contains(err.Error(), errOverLimit.Error()) {
					// Mark the transfer as needed manual approval for being over the limit(s).
					req.overAmountLimit = true
				} else {
					responder.Log("transfers", fmt.Sprintf("rejecting transfers: %v", err))
					responder.Problem(err)
					return
				}
			}

			// Post the Transfer's transaction against the Accounts
			if c.accountsClient != nil {
				tx, err := c.postAccountTransaction(responder.XUserID, origDep, receiverDep, req.Amount, req.Type, responder.XRequestID)
				if err != nil {
					responder.Log("transfers", err.Error())
					responder.Problem(err)
					return
				}
				req.transactionID = tx.ID
			}

			// Verify Customer statuses related to this transfer
			if c.customersClient != nil {
				// Pulling from a Receiver requires we've verified it already. Also, it can't be "credit only".
				if req.Type == model.PullTransfer {
					// TODO(adam): if receiver.Status == model.ReceiverCreditOnly
					if receiver.Status != model.ReceiverVerified {
						err = fmt.Errorf("receiver_id=%s is not Verified user_id=%s", receiver.ID, responder.XUserID)
						responder.Log("transfers", "problem with Receiver", "error", err.Error())
						responder.Problem(err)
						return
					}
				}
				// Check the related Customer objects for the Originator and Receiver
				if err := verifyCustomerStatuses(orig, receiver, c.customersClient, responder.XRequestID, responder.XUserID); err != nil {
					responder.Log("transfers", "problem with Customer checks", "error", err.Error())
					responder.Problem(err)
					return
				} else {
					responder.Log("transfers", "Customer check passed")
				}
				// Check any disclaimers for related Originator and Receiver
				if err := verifyDisclaimersAreAccepted(orig, receiver, c.customersClient, responder.XRequestID, responder.XUserID); err != nil {
					responder.Log("transfers", "problem with disclaimers", "error", err.Error())
					responder.Problem(err)
					return
				} else {
					responder.Log("transfers", "Disclaimer checks passed")
				}
			}

			// Save Transfer object
			transfer := req.asTransfer(transferID)

			// Verify the Transfer isn't pushed into "reviewable"
			if transfer.Status != model.TransferPending {
				err = fmt.Errorf("transfer_id=%s is not Pending (status=%s)", transfer.ID, transfer.Status)
				responder.Log("transfers", "can't process transfer", "error", err)
				responder.Problem(err)
				return
			}

			// Write events for our audit/history log
			if err := writeTransferEvent(responder.XUserID, req, c.eventRepo); err != nil {
				responder.Log("transfers", fmt.Sprintf("error writing transfer=%s event: %v", transferID, err))
				responder.Problem(err)
				return
			}
		}

		// TODO(adam): We still create Transfers if the micro-deposits have been confirmed, but not merged (and uploaded)
		// into an ACH file. Should we check that case in this method and reject Transfers whose Depositories micro-deposts
		// haven't even been merged yet?

		transfers, err := c.transferRepo.createUserTransfers(responder.XUserID, requests)
		if err != nil {
			responder.Log("transfers", fmt.Sprintf("error creating transfers: %v", err))
			responder.Problem(err)
			return
		}

		writeResponse(c.logger, w, len(requests), transfers)
		responder.Log("transfers", fmt.Sprintf("Created transfers for user_id=%s request=%s", responder.XUserID, responder.XRequestID))
	}
}

type RemoveTransferRequest struct {
	Transfer *model.Transfer

	XRequestID string
	XUserID    id.User

	Waiter chan interface{}
}

func (req *RemoveTransferRequest) send(controller chan interface{}) {
	req.Waiter = make(chan interface{}, 1)
	if controller != nil {
		controller <- req
		<-req.Waiter
	}
}

func (c *TransferRouter) deleteUserTransfer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		if responder == nil {
			return
		}

		transferID := getTransferID(r)
		transfer, err := c.transferRepo.getUserTransfer(transferID, responder.XUserID)
		if err != nil {
			responder.Log("transfers", fmt.Sprintf("error reading transfer=%s for deletion: %v", transferID, err))
			responder.Problem(err)
			return
		}
		if transfer.Status != model.TransferPending {
			responder.Problem(fmt.Errorf("a %s transfer can't be deleted", transfer.Status))
			return
		}

		// cancel and delete the transfer
		if err := c.transferRepo.UpdateTransferStatus(transferID, model.TransferCanceled); err != nil {
			responder.Problem(err)
			return
		}
		if err := c.transferRepo.deleteUserTransfer(transferID, responder.XUserID); err != nil {
			responder.Problem(err)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// getTransferObjects performs database lookups to grab all the objects needed to make a transfer.
//
// This method also verifies the status of the Receiver, Receiver Depository and Originator Repository
//
// All return values are either nil or non-nil and the error will be the opposite.
func (c *TransferRouter) getTransferObjects(
	userID id.User,
	origID model.OriginatorID,
	origDepID id.Depository,
	recID model.ReceiverID,
	recDepID id.Depository,
) (*model.Receiver, *model.Depository, *model.Originator, *model.Depository, error) {
	// Receiver
	receiver, err := c.receiverRepository.GetUserReceiver(recID, userID)
	if receiver == nil || err != nil {
		return nil, nil, nil, nil, fmt.Errorf("receiver not found: %v", err)
	}
	if err := receiver.Validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("receiver: %v", err)
	}

	receiverDep, err := c.depRepo.GetUserDepository(recDepID, userID)
	if receiverDep == nil || err != nil {
		return nil, nil, nil, nil, fmt.Errorf("receiver depository not found: %v", err)
	}
	if err := receiverDep.Validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("receiver depository: %v", err)
	}
	if receiverDep.Status != model.DepositoryVerified {
		return nil, nil, nil, nil, fmt.Errorf("receiver depository %s is in status %v", receiverDep.ID, receiverDep.Status)
	}

	// Originator
	orig, err := c.origRepo.GetUserOriginator(origID, userID)
	if orig == nil || err != nil {
		return nil, nil, nil, nil, fmt.Errorf("originator not found: %v", err)
	}
	if err := orig.Validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("originator: %v", err)
	}

	origDep, err := c.depRepo.GetUserDepository(origDepID, userID)
	if origDep == nil || err != nil {
		return nil, nil, nil, nil, fmt.Errorf("originator Depository not found: %v", err)
	}
	if err := origDep.Validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("originator depository: %v", err)
	}
	if origDep.Status != model.DepositoryVerified {
		return nil, nil, nil, nil, fmt.Errorf("originator Depository %s is in status %v", origDep.ID, origDep.Status)
	}

	return receiver, receiverDep, orig, origDep, nil
}

func writeResponse(logger log.Logger, w http.ResponseWriter, reqCount int, transfers []*model.Transfer) {
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

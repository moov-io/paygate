// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package originators

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type originatorRequest struct {
	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository id.Depository `json:"defaultDepository"`

	// Identification is a number by which the receiver is known to the originator
	Identification string `json:"identification"`

	// BirthDate is an optional value required for Know Your Customer (KYC) validation of this Originator
	BirthDate time.Time `json:"birthDate,omitempty"`

	// Address is an optional object required for Know Your Customer (KYC) validation of this Originator
	Address *model.Address `json:"address,omitempty"`

	// Metadata provides additional data to be used for display and search only
	Metadata string `json:"metadata"`

	// customerID is a unique ID from Moov's Customers service
	customerID string
}

func (r originatorRequest) missingFields() error {
	if r.Identification == "" {
		return errors.New("missing originatorRequest.Identification")
	}
	if r.DefaultDepository.String() == "" {
		return errors.New("missing originatorRequest.DefaultDepository")
	}
	return nil
}

func AddOriginatorRoutes(logger log.Logger, r *mux.Router, accountsClient accounts.Client, customersClient customers.Client, depositoryRepo depository.Repository, originatorRepo Repository) {
	r.Methods("GET").Path("/originators").HandlerFunc(getUserOriginators(logger, originatorRepo))
	r.Methods("POST").Path("/originators").HandlerFunc(createUserOriginator(logger, accountsClient, customersClient, depositoryRepo, originatorRepo))

	r.Methods("GET").Path("/originators/{originatorId}").HandlerFunc(getUserOriginator(logger, originatorRepo))
	r.Methods("DELETE").Path("/originators/{originatorId}").HandlerFunc(deleteUserOriginator(logger, originatorRepo))
}

func getUserOriginators(logger log.Logger, originatorRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		origs, err := originatorRepo.getUserOriginators(responder.XUserID)
		if err != nil {
			responder.Log("originators", fmt.Sprintf("problem reading user originators: %v", err))
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(origs)
		})
	}
}

func readOriginatorRequest(r *http.Request) (originatorRequest, error) {
	var wrapper originatorRequest
	if err := json.NewDecoder(route.Read(r.Body)).Decode(&wrapper); err != nil {
		return wrapper, err
	}
	if err := wrapper.missingFields(); err != nil {
		return wrapper, fmt.Errorf("%v: %v", route.ErrMissingRequiredJson, err)
	}
	return wrapper, nil
}

func createUserOriginator(logger log.Logger, accountsClient accounts.Client, customersClient customers.Client, depositoryRepo depository.Repository, originatorRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		req, err := readOriginatorRequest(r)
		if err != nil {
			responder.Log("originators", err.Error())
			responder.Problem(err)
			return
		}

		userID, requestID := route.GetUserID(r), moovhttp.GetRequestID(r)

		// Verify depository belongs to the user
		dep, err := depositoryRepo.GetUserDepository(req.DefaultDepository, userID)
		if err != nil || dep == nil || dep.ID != req.DefaultDepository {
			responder.Problem(fmt.Errorf("depository %s does not exist", req.DefaultDepository))
			return
		}

		// Verify account exists in Accounts for receiver (userID)
		if accountsClient != nil {
			account, err := accountsClient.SearchAccounts(requestID, userID, dep)
			if err != nil || account == nil {
				responder.Log("originators", fmt.Sprintf("problem finding account depository=%s: %v", dep.ID, err))
				responder.Problem(err)
				return
			}
		}

		// Create the customer with Moov's service
		if customersClient != nil {
			customer, err := customersClient.Create(&customers.Request{
				// Email: "foo@moov.io", // TODO(adam): should we include this? (and thus require it from callers)
				Name:      dep.Holder,
				BirthDate: req.BirthDate,
				Addresses: model.ConvertAddress(req.Address),
				SSN:       req.Identification,
				RequestID: responder.XRequestID,
				UserID:    responder.XUserID,
			})
			if err != nil || customer == nil {
				responder.Log("originators", "error creating Customer", "error", err)
				responder.Problem(err)
				return
			}
			responder.Log("originators", fmt.Sprintf("created customer=%s", customer.ID))
			req.customerID = customer.ID
		} else {
			responder.Log("originators", "skipped adding originator into Customers")
		}

		// Write Originator to DB
		orig, err := originatorRepo.createUserOriginator(userID, req)
		if err != nil {
			responder.Log("originators", fmt.Sprintf("problem creating originator: %v", err))
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(orig)
		})
	}
}

func getUserOriginator(logger log.Logger, originatorRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		origID := getOriginatorId(r)
		orig, err := originatorRepo.GetUserOriginator(origID, responder.XUserID)
		if err != nil {
			responder.Log("originators", fmt.Sprintf("problem reading originator=%s: %v", origID, err))
			responder.Problem(err)
			return
		}
		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(orig)
		})
	}
}

func deleteUserOriginator(logger log.Logger, originatorRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		origID := getOriginatorId(r)
		if err := originatorRepo.deleteUserOriginator(origID, responder.XUserID); err != nil {
			responder.Log("originators", fmt.Sprintf("problem deleting originator=%s: %v", origID, err))
			responder.Problem(err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func getOriginatorId(r *http.Request) model.OriginatorID {
	vars := mux.Vars(r)
	v, ok := vars["originatorId"]
	if ok {
		return model.OriginatorID(v)
	}
	return model.OriginatorID("")
}

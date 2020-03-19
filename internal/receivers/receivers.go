// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package receivers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"time"

	"github.com/moov-io/base"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type receiverRequest struct {
	Email             string         `json:"email,omitempty"`
	DefaultDepository id.Depository  `json:"defaultDepository,omitempty"`
	BirthDate         *time.Time     `json:"birthDate,omitempty"`
	Address           *model.Address `json:"address,omitempty"`
	Metadata          string         `json:"metadata,omitempty"`
}

func (r receiverRequest) missingFields() error {
	if r.Email == "" {
		return errors.New("missing receiverRequest.Email")
	}
	if r.DefaultDepository.String() == "" {
		return errors.New("missing receiverRequest.DefaultDepository")
	}
	return nil
}

func AddReceiverRoutes(logger log.Logger, r *mux.Router, customersClient customers.Client, depositoryRepo depository.Repository, receiverRepo Repository) {
	r.Methods("GET").Path("/receivers").HandlerFunc(getUserReceivers(logger, receiverRepo))
	r.Methods("POST").Path("/receivers").HandlerFunc(createUserReceiver(logger, customersClient, depositoryRepo, receiverRepo))

	r.Methods("GET").Path("/receivers/{receiverId}").HandlerFunc(GetUserReceiver(logger, receiverRepo))
	r.Methods("PATCH").Path("/receivers/{receiverId}").HandlerFunc(updateUserReceiver(logger, depositoryRepo, receiverRepo))
	r.Methods("DELETE").Path("/receivers/{receiverId}").HandlerFunc(deleteUserReceiver(logger, receiverRepo))
}

func getUserReceivers(logger log.Logger, receiverRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		receivers, err := receiverRepo.getUserReceivers(responder.XUserID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(receivers)
		})
	}
}

func readReceiverRequest(r *http.Request) (receiverRequest, error) {
	var wrapper receiverRequest
	if err := json.NewDecoder(route.Read(r.Body)).Decode(&wrapper); err != nil {
		return wrapper, err
	}
	if err := wrapper.missingFields(); err != nil {
		return wrapper, fmt.Errorf("%v: %v", route.ErrMissingRequiredJson, err)
	}
	return wrapper, nil
}

// parseAndValidateEmail attempts to parse an email address and validate the domain name.
func parseAndValidateEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return "", fmt.Errorf("error parsing '%s': %v", raw, err)
	}
	return addr.Address, nil
}

func createUserReceiver(logger log.Logger, customersClient customers.Client, depositoryRepo depository.Repository, receiverRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		req, err := readReceiverRequest(r)
		if err != nil {
			responder.Log("receivers", fmt.Errorf("error reading receiverRequest: %v", err))
			responder.Problem(err)
			return
		}

		dep, err := depositoryRepo.GetUserDepository(req.DefaultDepository, responder.XUserID)
		if err != nil || dep == nil {
			responder.Log("receivers", "depository not found")
			responder.Problem(errors.New("depository not found"))
			return
		}

		email, err := parseAndValidateEmail(req.Email)
		if err != nil {
			responder.Log("receivers", fmt.Sprintf("unable to validate receiver email: %v", err))
			responder.Problem(err)
			return
		}

		// Create our receiver
		receiver := &model.Receiver{
			ID:                model.ReceiverID(base.ID()),
			Email:             email,
			DefaultDepository: req.DefaultDepository,
			Status:            model.ReceiverUnverified,
			Metadata:          req.Metadata,
			Created:           base.NewTime(time.Now()),
		}
		if err := receiver.Validate(); err != nil {
			responder.Log("receivers", fmt.Errorf("error validating Receiver: %v", err))
			responder.Problem(err)
			return
		}

		// Add the Receiver into our Customers service
		if customersClient != nil {
			opts := &customers.Request{
				Name:      dep.Holder,
				Addresses: model.ConvertAddress(req.Address),
				Email:     email,
				RequestID: responder.XRequestID,
				UserID:    responder.XUserID,
			}
			if req.BirthDate != nil {
				opts.BirthDate = *req.BirthDate
			}
			customer, err := customersClient.Create(opts)
			if err != nil || customer == nil {
				responder.Log("receivers", "error creating customer", "error", err)
				responder.Problem(err)
				return
			}
			responder.Log("receivers", fmt.Sprintf("created customer=%s", customer.ID))
			receiver.CustomerID = customer.ID
		} else {
			responder.Log("receivers", "skipped adding receiver into Customers")
		}

		if err := receiverRepo.UpsertUserReceiver(responder.XUserID, receiver); err != nil {
			err = fmt.Errorf("creating receiver=%s, user_id=%s: %v", receiver.ID, responder.XUserID, err)
			responder.Log("receivers", fmt.Errorf("error inserting Receiver: %v", err))
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(receiver)
		})
	}
}

func GetUserReceiver(logger log.Logger, receiverRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		receiverID := getReceiverID(r)
		if receiverID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver, err := receiverRepo.GetUserReceiver(receiverID, responder.XUserID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(receiver)
		})
	}
}

func updateUserReceiver(logger log.Logger, depRepo depository.Repository, receiverRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		var wrapper receiverRequest
		if err := json.NewDecoder(route.Read(r.Body)).Decode(&wrapper); err != nil {
			responder.Problem(err)
			return
		}

		receiverID := getReceiverID(r)
		if receiverID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver, err := receiverRepo.GetUserReceiver(receiverID, responder.XUserID)
		if receiver == nil || err != nil {
			responder.Log("receivers", fmt.Sprintf("problem getting receiver='%s': %v", receiverID, err))
			responder.Problem(err)
			return
		}
		if wrapper.DefaultDepository != "" {
			// Verify the user controls the requested Depository
			dep, err := depRepo.GetUserDepository(wrapper.DefaultDepository, responder.XUserID)
			if err != nil || dep == nil {
				responder.Log("receivers", "depository doesn't belong to user")
				responder.Problem(errors.New("depository not found"))
				return
			}
			receiver.DefaultDepository = wrapper.DefaultDepository
		}
		if wrapper.Metadata != "" {
			receiver.Metadata = wrapper.Metadata
		}
		receiver.Updated = base.NewTime(time.Now())

		if err := receiver.Validate(); err != nil {
			responder.Log("receivers", fmt.Sprintf("problem validating updatable receiver=%s: %v", receiver.ID, err))
			responder.Problem(err)
			return
		}

		// Perform update
		if err := receiverRepo.UpsertUserReceiver(responder.XUserID, receiver); err != nil {
			responder.Log("receivers", fmt.Sprintf("problem upserting receiver=%s: %v", receiver.ID, err))
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(receiver)
		})
	}
}

func deleteUserReceiver(logger log.Logger, receiverRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if responder == nil {
			return
		}

		if receiverID := getReceiverID(r); receiverID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			if err := receiverRepo.deleteUserReceiver(receiverID, responder.XUserID); err != nil {
				responder.Problem(err)
				return
			}
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
		})
	}
}

// getReceiverID extracts the ReceiverID from the incoming request.
func getReceiverID(r *http.Request) model.ReceiverID {
	v := mux.Vars(r)
	id, ok := v["receiverId"]
	if !ok {
		return model.ReceiverID("")
	}
	return model.ReceiverID(id)
}

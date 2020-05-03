// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"
	"github.com/moov-io/paygate/pkg/util"
	"github.com/moov-io/paygate/x/route"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type Router struct {
	Logger log.Logger
	Repo   Repository

	Publisher pipeline.XferPublisher

	GetUserTransfers   http.HandlerFunc
	CreateUserTransfer http.HandlerFunc
	GetUserTransfer    http.HandlerFunc
	DeleteUserTransfer http.HandlerFunc
}

func NewRouter(logger log.Logger, repo Repository, fundStrategy fundflow.Strategy, pub pipeline.XferPublisher) *Router {
	return &Router{
		Logger:             logger,
		Repo:               repo,
		Publisher:          pub,
		GetUserTransfers:   GetUserTransfers(logger, repo),
		CreateUserTransfer: CreateUserTransfer(logger, repo, fundStrategy, pub),
		GetUserTransfer:    GetUserTransfer(logger, repo),
		DeleteUserTransfer: DeleteUserTransfer(logger, repo, pub),
	}
}

func (c *Router) RegisterRoutes(r *mux.Router) {
	r.Methods("GET").Path("/transfers").HandlerFunc(c.GetUserTransfers)
	r.Methods("POST").Path("/transfers").HandlerFunc(c.CreateUserTransfer)
	r.Methods("GET").Path("/transfers/{transferID}").HandlerFunc(c.GetUserTransfer)
	r.Methods("DELETE").Path("/transfers/{transferID}").HandlerFunc(c.DeleteUserTransfer)
}

func getTransferID(r *http.Request) string {
	return route.ReadPathID("transferID", r)
}

type transferFilterParams struct {
	Status    client.TransferStatus
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
	if r.URL != nil {
		q := r.URL.Query()
		if v := q.Get("startDate"); v != "" {
			params.StartDate = util.FirstParsedTime(v, base.ISO8601Format, util.YYMMDDTimeFormat)
		}
		if v := q.Get("endDate"); v != "" {
			params.EndDate, _ = time.Parse(base.ISO8601Format, v)
			fmt.Printf("params.EndDate=%v\n", params.EndDate)
		}
		if s := strings.TrimSpace(q.Get("status")); s != "" {
			params.Status = client.TransferStatus(s)
		}
	}
	if limit := route.ReadLimit(r); limit != 0 {
		params.Limit = limit
	}
	if offset := route.ReadOffset(r); offset != 0 {
		params.Offset = offset
	}
	return params
}

func GetUserTransfers(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		params := readTransferFilterParams(r)
		xfers, err := repo.getUserTransfers(responder.XUserID, params)
		if err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(xfers)
		})
	}
}

func CreateUserTransfer(logger log.Logger, repo Repository, fundStrategy fundflow.Strategy, pub pipeline.XferPublisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		var req client.CreateTransfer
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			responder.Problem(err)
			return
		}

		transfer := &client.Transfer{
			TransferID:  base.ID(),
			Amount:      req.Amount,
			Source:      req.Source,
			Destination: req.Destination,
			Description: req.Description,
			Status:      client.PENDING,
			SameDay:     req.SameDay,
			Created:     time.Now(),
		}

		// TODO(adam): validate both Customer and get get their Accounts
		// get/decrypt destination account number
		//
		// TODO(adam): future: limits checks

		// Save our Transfer to the database
		if err := repo.writeUserTransfers(responder.XUserID, transfer); err != nil {
			responder.Problem(err)
			return
		}

		// According to our strategy create (originate) ACH files to be published somewhere
		if fundStrategy != nil {
			files, err := fundStrategy.Originate(transfer, fundflow.Source{}, fundflow.Destination{})
			if err != nil {
				responder.Problem(err)
				return
			}
			if err := pipeline.PublishFiles(pub, transfer, files); err != nil {
				responder.Problem(err)
				return
			}
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(transfer)
		})
	}
}

func GetUserTransfer(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		xfer, err := repo.GetTransfer(getTransferID(r))
		if err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(xfer)
		})
	}
}

func DeleteUserTransfer(logger log.Logger, repo Repository, pub pipeline.XferPublisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		if pub != nil {
			err := pub.Cancel(pipeline.Xfer{
				Transfer: &client.Transfer{
					TransferID: getTransferID(r),
				},
			})
			if err != nil {
				responder.Problem(err)
				return
			}
		}

		transferID := getTransferID(r)
		if err := repo.deleteUserTransfer(responder.XUserID, transferID); err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
		})
	}
}

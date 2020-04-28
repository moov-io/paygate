// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/transfers/offload"
	"github.com/moov-io/paygate/x/route"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type Router struct {
	Logger log.Logger
	Repo   Repository

	Offloader offload.Offloader

	GetUserTransfers   http.HandlerFunc
	CreateUserTransfer http.HandlerFunc
	GetUserTransfer    http.HandlerFunc
	DeleteUserTransfer http.HandlerFunc
}

func NewRouter(logger log.Logger, repo Repository, off offload.Offloader) *Router {
	return &Router{
		Logger:             logger,
		Repo:               repo,
		Offloader:          off,
		GetUserTransfers:   GetUserTransfers(logger, repo),
		CreateUserTransfer: CreateUserTransfer(logger, repo, off),
		GetUserTransfer:    GetUserTransfer(logger, repo),
		DeleteUserTransfer: DeleteUserTransfer(logger, repo, off),
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

func GetUserTransfers(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*client.Transfer{
				{
					TransferID: base.ID(),
					Amount:     "USD 12.45",
					Source: client.Source{
						CustomerID: base.ID(),
						AccountID:  base.ID(),
					},
					Destination: client.Destination{
						CustomerID: base.ID(),
						AccountID:  base.ID(),
					},
					Description: "payroll",
					Status:      client.PENDING,
					SameDay:     false,
					Created:     time.Now(),
				},
			})
		})
	}
}

func CreateUserTransfer(logger log.Logger, repo Repository, off offload.Offloader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		var req client.CreateTransfer
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			responder.Problem(err)
			return
		}

		// create file, write to ./storage/incoming/
		// controller periodically grabs those and

		transfer := &client.Transfer{
			TransferID: base.ID(),
			Amount:     "USD 12.45",
			Source: client.Source{
				CustomerID: base.ID(),
				AccountID:  base.ID(),
			},
			Destination: client.Destination{
				CustomerID: base.ID(),
				AccountID:  base.ID(),
			},
			Description: "payroll",
			Status:      client.PENDING,
			SameDay:     false,
			Created:     time.Now(),
		}

		if off != nil {
			err := off.Upload(offload.Xfer{
				Transfer: transfer,
			})
			if err != nil {
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

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&client.Transfer{
				TransferID: base.ID(),
				Amount:     "USD 12.45",
				Source: client.Source{
					CustomerID: base.ID(),
					AccountID:  base.ID(),
				},
				Destination: client.Destination{
					CustomerID: base.ID(),
					AccountID:  base.ID(),
				},
				Description: "payroll",
				Status:      client.PENDING,
				SameDay:     false,
				Created:     time.Now(),
			})
		})
	}
}

func DeleteUserTransfer(logger log.Logger, repo Repository, off offload.Offloader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		if off != nil {
			err := off.Cancel(offload.Xfer{
				Transfer: &client.Transfer{
					TransferID: getTransferID(r),
				},
			})
			if err != nil {
				responder.Problem(err)
				return
			}
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
		})
	}
}

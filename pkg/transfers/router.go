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
	"github.com/moov-io/paygate/pkg/transfers/offload"
	"github.com/moov-io/paygate/pkg/util"
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

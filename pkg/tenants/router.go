// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/x/route"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type Router struct {
	Logger log.Logger
	Repo   Repository

	GetUserTenants http.HandlerFunc
	UpdateTenant   http.HandlerFunc
}

func NewRouter(logger log.Logger, repo Repository) *Router {
	return &Router{
		Logger:         logger,
		Repo:           repo,
		GetUserTenants: GetUserTenants(logger, repo),
		UpdateTenant:   UpdateTenant(logger, repo),
	}
}

func (c *Router) RegisterRoutes(r *mux.Router) {
	r.Methods("GET").Path("/tenants").HandlerFunc(c.GetUserTenants)
	r.Methods("PUT").Path("/tenants/{tenantID}").HandlerFunc(c.UpdateTenant)
}

func GetUserTenants(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		tenants, err := repo.List(responder.XUserID)
		if err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(tenants)
		})
	}
}

func UpdateTenant(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		tenantID := route.ReadPathID("tenantID", r)
		if tenantID == "" {
			responder.Problem(errors.New("missing tenantID"))
			return
		}

		var req client.UpdateTenant
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			responder.Problem(err)
			return
		}

		if err := repo.UpdateTenant(tenantID, req); err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
		})
	}
}

// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"encoding/json"
	"net/http"

	"github.com/moov-io/base"
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

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*client.Tenant{
				{
					TenantID:        base.ID(),
					Name:            "My Tenant",
					PrimaryCustomer: "foo",
				},
			})
		})
	}
}

func UpdateTenant(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
		})
	}
}

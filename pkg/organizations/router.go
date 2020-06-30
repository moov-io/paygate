// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organizations

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/x/route"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type Router struct {
	GetOrganizations   http.HandlerFunc
	CreateOrganization http.HandlerFunc
	UpdateOrganization http.HandlerFunc
}

func NewRouter(logger log.Logger, repo Repository) *Router {
	return &Router{
		GetOrganizations:   GetOrganizations(logger, repo),
		CreateOrganization: CreateOrganization(logger, repo),
		UpdateOrganization: UpdateOrganization(logger, repo),
	}
}

func (c *Router) RegisterRoutes(r *mux.Router) {
	r.Methods("GET").Path("/organizations").HandlerFunc(c.GetOrganizations)
	r.Methods("POST").Path("/organizations").HandlerFunc(c.CreateOrganization)
	r.Methods("PUT").Path("/organizations/{organizationID}").HandlerFunc(c.UpdateOrganization)
}

func getOrganizationID(r *http.Request) string {
	return route.ReadPathID("organizationID", r)
}

func validateOrganization(org client.Organization) error {
	if org.Name == "" {
		return errors.New("missing name")
	}
	if org.PrimaryCustomer == "" {
		return errors.New("missing primary customer")
	}
	if org.TenantID == "" {
		return errors.New("missing tenantID")
	}
	return nil
}

func GetOrganizations(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		orgs, err := repo.getOrganizations(responder.XUserID)
		if err != nil {
			responder.Problem(err)
			return
		}
		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(orgs)
		})
	}
}

func CreateOrganization(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		var org client.Organization
		if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
			responder.Problem(err)
			return
		}
		if err := validateOrganization(org); err != nil {
			responder.Problem(err)
			return
		}
		org.OrganizationID = base.ID()

		userID := route.HeaderUserID(r)
		if err := repo.createOrganization(userID, org); err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(org)
		})
	}
}

func UpdateOrganization(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		var org client.Organization
		if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
			responder.Problem(err)
			return
		}
		if org.Name == "" {
			responder.Problem(errors.New("missing name"))
			return
		}

		if err := repo.updateOrganizationName(getOrganizationID(r), org.Name); err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
		})
	}
}

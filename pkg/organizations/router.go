// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organizations

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/client"
	"github.com/moov-io/paygate/x/route"
	"github.com/moov-io/paygate/x/trace"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type Router struct {
	logger log.Logger
	repo   Repository
}

func NewRouter(logger log.Logger, repo Repository) *Router {
	return &Router{
		logger: logger,
		repo:   repo,
	}
}

func (c *Router) RegisterRoutes(r *mux.Router) {
	r.Methods("GET").Path("/organizations").HandlerFunc(c.getOrganizations())
	r.Methods("POST").Path("/organizations").HandlerFunc(c.createOrganization())
	r.Methods("PUT").Path("/organizations/{organizationID}").HandlerFunc(c.updateOrganization())
}

func getOrganizationID(r *http.Request) string {
	return route.ReadPathID("organizationID", r)
}

func (c *Router) getOrganizations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		span := responder.Span()
		defer span.Finish()

		orgs, err := c.repo.getOrganizations(route.HeaderUserID(r))
		if err != nil {
			responder.Problem(err)
			return
		}

		req, _ := http.NewRequest("GET", "/foo", nil)
		req = trace.DecorateHttpRequest(req, span)
		fmt.Printf("%#v\n", req.Header)

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(orgs)
		})
	}
}

func validateOrganization(org client.Organization) error {
	if org.Name == "" {
		return errors.New("missing name")
	}
	if org.PrimaryCustomer == "" {
		return errors.New("missing primary customer")
	}
	return nil
}

func (c *Router) createOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		span := responder.Span()
		defer span.Finish()

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
		if err := c.repo.createOrganization(userID, org); err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(org)
		})
	}
}

func (c *Router) updateOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(c.logger, w, r)
		span := responder.Span()
		defer span.Finish()

		var org client.Organization
		if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
			responder.Problem(err)
			return
		}
		if org.Name == "" {
			responder.Problem(errors.New("missing name"))
			return
		}

		if err := c.repo.updateOrganizationName(getOrganizationID(r), org.Name); err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
		})
	}
}

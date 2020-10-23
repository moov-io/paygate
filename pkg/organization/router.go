// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organization

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/x/route"
)

type Router struct {
	GetConfig    http.HandlerFunc
	UpdateConfig http.HandlerFunc
}

func NewRouter(orgRepo Repository) *Router {
	return &Router{
		GetConfig:    getConfig(orgRepo),
		UpdateConfig: updateConfig(orgRepo),
	}
}

func (router *Router) RegisterRoutes(r *mux.Router) {
	r.Methods("PUT").Path("/configuration/transfers").HandlerFunc(router.UpdateConfig)
	r.Methods("GET").Path("/configuration/transfers").HandlerFunc(router.GetConfig)
}

func getConfig(repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organization := route.GetHeaderValue("X-Organization", r)
		if organization == "" {
			moovhttp.Problem(w, errors.New("missing organization"))
			return
		}

		cfg, err := repo.GetConfig(organization)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(cfg)
	}
}

func updateConfig(repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organization := route.GetHeaderValue("X-Organization", r)
		if organization == "" {
			moovhttp.Problem(w, errors.New("missing organization"))
			return
		}
		var body client.OrganizationConfiguration
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		cfg, err := repo.UpdateConfig(organization, &body)
		if err != nil {
			moovhttp.Problem(w, fmt.Errorf("problem updating config - error=%v", err))
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(cfg)
	}
}

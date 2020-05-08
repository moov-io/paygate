// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/admin"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/tenants"
	"github.com/moov-io/paygate/x/route"

	"github.com/go-kit/kit/log"
)

func createTenant(logger log.Logger, repo tenants.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		if r.Method != http.MethodPost {
			responder.Problem(fmt.Errorf("invalid method %s", r.Method))
			return
		}

		var req admin.CreateTenant
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			responder.Problem(err)
			return
		}

		tenant := client.Tenant{
			TenantID:        base.ID(),
			Name:            req.Name,
			PrimaryCustomer: req.PrimaryCustomer,
		}
		if err := validateTenant(tenant); err != nil {
			responder.Problem(err)
			return
		}
		companyIdentification := tenants.CompanyIdentification("MOOV") // TODO(adam): read from config
		if err := repo.Create(responder.XUserID, companyIdentification, tenant); err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(tenant)
		})
	}
}

func validateTenant(tenant client.Tenant) error {
	if tenant.Name == "" {
		return errors.New("missing Tenant name")
	}
	if tenant.PrimaryCustomer == "" {
		return errors.New("missing PrimaryCustomer")
	}
	return nil
}

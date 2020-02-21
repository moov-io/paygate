// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package features

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/moov-io/base/admin"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/route"

	"github.com/go-kit/kit/log"
)

func AddRoutes(logger log.Logger, svc *admin.Server, accountsCallsDisabled, customersCallsDisabled bool) {
	svc.AddHandler("/features", getCurrentFeatures(logger, accountsCallsDisabled, customersCallsDisabled))
}

type response struct {
	AccountsCallsDisabled  bool `json:"accountsCallsDisabled"`
	CustomersCallsDisabled bool `json:"customersCallsDisabled"`
}

func getCurrentFeatures(logger log.Logger, accountsCallsDisabled, customersCallsDisabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w = route.Wrap(logger, w, r)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if r.Method != "GET" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb: %s", r.Method))
			return
		}

		resp := &response{
			AccountsCallsDisabled:  accountsCallsDisabled,
			CustomersCallsDisabled: customersCallsDisabled,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

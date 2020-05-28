// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"encoding/json"
	"net/http"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/pkg/config"
)

// RegisterRoutes will add HTTP handlers for PayGate's admin HTTP server
func RegisterRoutes(svc *admin.Server, cfg *config.Config) {
	if cfg.Admin.DisableConfigEndpoint {
		return
	}

	svc.AddHandler("/config", marshalConfig(cfg))
}

func marshalConfig(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(cfg)
	}
}

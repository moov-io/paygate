// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package auth

import (
	"net/http"

	"github.com/moov-io/paygate/pkg/config"
)

type TenantExtractor interface {
	TenantID(req *http.Request) string
}

func NewExtractor(cfg *config.Config) (TenantExtractor, error) {
	if cfg.Auth.Tenants.Tumbler != nil {
		return newTumblerExtractor(cfg.Logger, cfg.Auth.Tenants.Tumbler)
	}
	return newHeaderExtractor(cfg.Auth.Tenants)
}

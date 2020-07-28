// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package auth

import (
	"net/http"

	"github.com/moov-io/paygate/pkg/config"
)

type headerExtractor struct {
	headerNames []string
}

func newHeaderExtractor(cfg config.Tenants) (*headerExtractor, error) {
	var names []string
	if cfg.Headers != nil {
		names = cfg.Headers.Names
	}
	return &headerExtractor{
		headerNames: names,
	}, nil
}

func (ex *headerExtractor) TenantID(req *http.Request) string {
	for i := range ex.headerNames {
		if v := req.Header.Get(ex.headerNames[i]); v != "" {
			return v
		}
	}
	return ""
}

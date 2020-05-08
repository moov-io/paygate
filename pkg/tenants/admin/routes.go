// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/pkg/tenants"

	"github.com/go-kit/kit/log"
)

// RegisterRoutes will add HTTP handlers for paygate's admin HTTP server
func RegisterRoutes(logger log.Logger, svc *admin.Server, repo tenants.Repository) {
	svc.AddHandler("/tenants", createTenant(logger, repo))
}

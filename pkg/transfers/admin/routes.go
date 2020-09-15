// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/transfers"
)

// RegisterRoutes will add HTTP handlers for paygate's admin HTTP server
func RegisterRoutes(cfg *config.Config, svc *admin.Server, repo transfers.Repository) {
	svc.AddHandler("/transfers/{transferId}/status", updateTransferStatus(cfg, repo))
}

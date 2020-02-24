// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"github.com/go-kit/kit/log"
	"github.com/moov-io/base/admin"
)

func RegisterAdminRoutes(logger log.Logger, svc *admin.Server, depRepo Repository) {
	svc.AddHandler("/depositories/{depositoryId}", overrideDepositoryStatus(logger, depRepo))

	svc.AddHandler("/depositories/{depositoryId}/micro-deposits", getMicroDeposits(logger, depRepo))
}

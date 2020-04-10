// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"database/sql"
	"os"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/gateways"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/originators"
	"github.com/moov-io/paygate/internal/receivers"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

type TestRouter struct {
	*TransferRouter

	depositoryRepo *depository.MockRepository
	eventRepo      *events.TestRepository
	gatewayRepo    *gateways.MockRepository
	originatorRepo *originators.MockRepository
	receiverRepo   *receivers.MockRepository
}

func setupTestRouter(t *testing.T, xferRepo Repository) *TestRouter {
	depRepo := &depository.MockRepository{}
	eventRepo := &events.TestRepository{}
	gatewayRepo := &gateways.MockRepository{
		Gateway: &model.Gateway{
			ID:              model.GatewayID(base.ID()),
			Origin:          "987654320",
			OriginName:      "My Bank",
			Destination:     "123456780", // TODO(adam): use valid routing number
			DestinationName: "Their Bank",
		},
	}
	origRepo := &originators.MockRepository{}
	recRepo := &receivers.MockRepository{}

	// set transfer Limiter
	var db *sql.DB
	if rr, ok := xferRepo.(*SQLRepo); ok {
		db = rr.db
	}
	limits, _ := ParseLimits(OneDayLimit(), SevenDayLimit(), ThirtyDayLimit())
	limiter := NewLimitChecker(log.NewNopLogger(), db, limits)

	accountsClient := &accounts.MockClient{}

	return &TestRouter{
		TransferRouter: &TransferRouter{
			// logger:               log.NewNopLogger(),
			logger:               log.NewLogfmtLogger(os.Stderr),
			depRepo:              depRepo,
			eventRepo:            eventRepo,
			gatewayRepo:          gatewayRepo,
			origRepo:             origRepo,
			receiverRepository:   recRepo,
			transferRepo:         xferRepo,
			transferLimitChecker: limiter,
			accountsClient:       accountsClient,
		},
		depositoryRepo: depRepo,
		eventRepo:      eventRepo,
		gatewayRepo:    gatewayRepo,
		originatorRepo: origRepo,
		receiverRepo:   recRepo,
	}
}

func (r *TestRouter) makeDepository(t *testing.T, depositoryID id.Depository) *model.Depository {
	dep := &model.Depository{
		ID:            depositoryID,
		Holder:        "John Doe",
		HolderType:    model.Individual,
		Type:          model.Checking,
		Status:        model.DepositoryVerified,
		RoutingNumber: "987654320",
		Keeper:        secrets.TestStringKeeper(t),
	}
	dep.ReplaceAccountNumber("123")
	r.depositoryRepo.Depositories = append(r.depositoryRepo.Depositories, dep)
	return dep
}

func (r *TestRouter) makeOriginator(originatorID model.OriginatorID) *model.Originator {
	orig := &model.Originator{
		ID:                originatorID,
		Identification:    "secret",
		DefaultDepository: id.Depository(base.ID()),
	}
	r.originatorRepo.Originators = append(r.originatorRepo.Originators, orig)
	return orig
}

func (r *TestRouter) makeReceiver(receiverID model.ReceiverID) *model.Receiver {
	rec := &model.Receiver{
		ID:                receiverID,
		Email:             "test@moov.io",
		DefaultDepository: id.Depository(base.ID()),
		Metadata:          "Jane Doe",
		Status:            model.ReceiverVerified,
	}
	r.receiverRepo.Receivers = append(r.receiverRepo.Receivers, rec)
	return rec
}

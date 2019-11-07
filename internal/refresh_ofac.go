// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moov-io/base"
	moovcustomers "github.com/moov-io/customers"
	"github.com/moov-io/paygate/internal/customers"

	"github.com/go-kit/kit/log"
)

type Refresher interface {
	Start(interval time.Duration) error
	Close()
}

func NewRefresher(logger log.Logger, client customers.Client, db *sql.DB) Refresher {
	ctx, shutdown := context.WithCancel(context.Background())

	staleness := 5 * time.Minute // TODO(adam): config with monthly default
	batchSize := 25              // TODO(adam): config with default of 25? 50?

	return &periodicRefresher{
		logger:           logger,
		client:           client,
		cur:              customers.NewCursor(logger, db, batchSize),
		minimumStaleness: staleness,
		ctx:              ctx,
		shutdown:         shutdown,
	}
}

type periodicRefresher struct {
	logger log.Logger

	client customers.Client
	cur    *customers.Cursor // needs access to 'originators' and 'receivers' tables

	depRepo      DepositoryRepository
	receiverRepo receiverRepository

	// minimumStaleness is how long ago a Customer's OFAC search can be before it needs
	// a refresh. Typically this is weekly or monthly depending on the business needs.
	minimumStaleness time.Duration

	ctx      context.Context
	shutdown context.CancelFunc
}

func (r *periodicRefresher) Start(interval time.Duration) error {
	if r == nil || r.client == nil {
		return errors.New("nil periodicRefresher or Customers client")
	}

	tick := time.NewTicker(interval)
	r.logger.Log("customers", fmt.Sprintf("refreshing customer OFAC searches every %v", interval))

	for {
		select {
		case <-tick.C:
			requestID := base.ID()
			customers, err := r.cur.Next()
			if err != nil {
				r.logger.Log("customers", fmt.Sprintf("cursor error: %v", err), "requestID", requestID)
				continue
			}
			r.logger.Log("customers", fmt.Sprintf("got %d customer records to refresh OFAC searches", len(customers)), "requestID", requestID)
			for i := range customers {
				if err := r.refreshSearchIfNeeded(customers[i], requestID); err != nil {
					r.logger.Log("customers", err.Error(), "requestID", requestID)
				}
			}

		case <-r.ctx.Done():
			r.logger.Log("customers", "periodicRefresher: shutdown")
			return nil
		}
	}
}

func (r *periodicRefresher) refreshSearchIfNeeded(cust customers.Cust, requestID string) error {
	result, err := r.client.LatestOFACSearch(cust.ID, requestID, "")
	if err != nil {
		return fmt.Errorf("error getting latest ofac search for customer=%s", cust.ID)
	}
	if searchIsOldEnough(result.CreatedAt, r.minimumStaleness) {
		r.logger.Log("customers", fmt.Sprintf("refreshing customer=%s ofac search", cust.ID), "requestID", requestID)

		if _, err := r.client.RefreshOFACSearch(cust.ID, "requestID", "userID"); err != nil {
			return fmt.Errorf("error refreshing ofac search for customer=%s: %v", cust.ID, err)
		}

		if err := rejectRelatedCustomerObjects(r.client, cust, requestID, r.depRepo, r.receiverRepo); err != nil {
			return fmt.Errorf("error rejecting customer=%s: %v", cust.ID, err)
		}
	}
	return nil
}

func searchIsOldEnough(when time.Time, staleness time.Duration) bool {
	return when.Add(staleness).Before(time.Now())
}

func rejectRelatedCustomerObjects(client customers.Client, c customers.Cust, requestID string, depRepo DepositoryRepository, receiverRepo receiverRepository) error {
	cust, err := client.Lookup(c.ID, requestID, "")
	if err != nil {
		return fmt.Errorf("error looking up customer=%s: %v", c.ID, err)
	}
	if status, err := moovcustomers.LiftStatus(cust.Status); status == nil || err != nil {
		return fmt.Errorf("error lifting customer=%s status: %v", c.ID, err)
	} else {
		if *status == moovcustomers.Rejected {
			if c.OriginatorID != "" {
				if err := depRepo.UpdateDepositoryStatus(DepositoryID(c.OriginatorDepository), DepositoryRejected); err != nil {
					return fmt.Errorf("error updating originator depository=%s: %v", c.OriginatorDepository, err)
				}
			} else {
				if err := receiverRepo.updateReceiverStatus(ReceiverID(c.ReceiverID), ReceiverSuspended); err != nil {
					return fmt.Errorf("error updating receiver=%s: %v", c.ReceiverID, err)
				}
			}
		}
	}
	return nil
}

func (r *periodicRefresher) Close() {
	if r == nil {
		return
	}
	r.shutdown()
}

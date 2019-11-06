// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
)

type Refresher interface {
	Start(interval time.Duration) error
	Close()
}

func NewRefresher(logger log.Logger, client Client) Refresher {
	ctx, shutdown := context.WithCancel(context.Background())
	return &periodicRefresher{
		logger:   logger,
		client:   client,
		ctx:      ctx,
		shutdown: shutdown,
	}
}

type periodicRefresher struct {
	logger log.Logger

	client Client

	ctx      context.Context
	shutdown context.CancelFunc
}

func (r *periodicRefresher) Start(interval time.Duration) error {
	if r == nil || r.client == nil {
		return errors.New("nil periodicRefresher or Customers client")
	}

	tick := time.NewTicker(interval)
	r.logger.Log("customers", fmt.Sprintf("refreshing customer OFAC searches every %v", interval))

	// TODO(adam): notes
	//
	// We'll need to scan Receiver and Originator customer_id's to refresh their ofac search monthly (will be a config).
	// After the refresh reject the object if Customer.Status == Rejected

	for {
		select {
		case <-tick.C:
			r.logger.Log("customers", "periodicRefresher: tick")

			// LatestOFACSearch(customerID, requestID, userID string) (*moovcustomers.OfacSearch, error)
			// RefreshOFACSearch(customerID, requestID, userID string) (*moovcustomers.Customer, error)

			customerID := "76f6dd6493387588a5127e209ab3705fee9e7d4a"

			result, err := r.client.LatestOFACSearch(customerID, "requestID", "userID")
			if err != nil {
				r.logger.Log("customers", fmt.Sprintf("AA: error=%v", err))
				continue
			}
			r.logger.Log("customers", fmt.Sprintf("OFAC refresh: %#v", result))

			result, err = r.client.RefreshOFACSearch(customerID, "requestID", "userID")
			if err != nil {
				r.logger.Log("customers", fmt.Sprintf("AB: error=%v", err))
				continue
			}
			r.logger.Log("customers", fmt.Sprintf("latest customer OFAC search: %#v", result))

		case <-r.ctx.Done():
			r.logger.Log("customers", "periodicRefresher: shutdown")
			return nil
		}
	}
}

func (r *periodicRefresher) Close() {
	if r == nil {
		return
	}
	r.shutdown()
}

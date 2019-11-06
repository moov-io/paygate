// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
)

type Refresher interface {
	Start(interval time.Duration) error
	Close()
}

func NewRefresher(logger log.Logger) Refresher {
	ctx, shutdown := context.WithCancel(context.Background())
	return &periodicRefresher{
		logger:   logger,
		ctx:      ctx,
		shutdown: shutdown,
	}
}

type periodicRefresher struct {
	logger log.Logger

	ctx      context.Context
	shutdown context.CancelFunc
}

func (r *periodicRefresher) Start(interval time.Duration) error {
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

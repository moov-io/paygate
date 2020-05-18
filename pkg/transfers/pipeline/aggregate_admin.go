// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"fmt"
	"net/http"

	"github.com/moov-io/base/admin"
	moovhttp "github.com/moov-io/base/http"
)

func (xfagg *XferAggregator) RegisterRoutes(svc *admin.Server) {
	svc.AddHandler("/trigger-cutoff", xfagg.triggerManualCutoff())
}

type manuallyTriggeredCutoff struct {
	C chan error
}

func (xfagg *XferAggregator) triggerManualCutoff() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			moovhttp.Problem(w, fmt.Errorf("invalid method %s", r.Method))
			return
		}

		// send off the manual request
		waiter := manuallyTriggeredCutoff{
			C: make(chan error, 1),
		}
		xfagg.cutoffTrigger <- waiter

		if err := <-waiter.C; err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			moovhttp.Problem(w, err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

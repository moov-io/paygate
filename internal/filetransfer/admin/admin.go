// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"net/http"
	"time"

	"github.com/moov-io/base/admin"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/util"

	"github.com/go-kit/kit/log"
)

func RegisterAdminRoutes(logger log.Logger, svc *admin.Server, flushIncoming FlushChan, flushOutgoing FlushChan) {
	// Endpoints to merge and flush incoming/outgoing files
	svc.AddHandler("/files/flush/incoming", flushIncomingFiles(logger, flushIncoming))
	svc.AddHandler("/files/flush/outgoing", flushOutgoingFiles(logger, flushOutgoing))
	svc.AddHandler("/files/flush", flushFiles(logger, flushIncoming, flushOutgoing))

	// Endpoint to just merge files, not upload
	svc.AddHandler("/files/merge", mergeOutgoingFiles(logger, flushOutgoing))
}

type FlushChan chan *Request

type Request struct {
	RequestID string
	UserID    string

	// SkipUpload will signal to only merge transfers and micro-deposits
	SkipUpload bool

	// Waiter is an optional channel to signal when the file operations
	// are completed. This is used to hold HTTP responses (for the admin
	// endpoints).
	Waiter chan struct{}
}

func maybeWaiter(r *http.Request) *Request {
	req := &Request{
		RequestID: moovhttp.GetRequestID(r),
		UserID:    moovhttp.GetUserID(r),
	}
	if _, exists := r.URL.Query()["wait"]; exists {
		req.Waiter = make(chan struct{}, 1)
	}
	return req
}

func maybeWait(w http.ResponseWriter, req *Request) error {
	if req.Waiter != nil {
		err := util.Timeout(func() error {
			<-req.Waiter // wait for a response from StartPeriodicFileOperations
			return nil
		}, 30*time.Second)

		if err == util.ErrTimeout {
			moovhttp.Problem(w, err)
			return err
		}
	}
	return nil
}

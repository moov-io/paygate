// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"net/http"
	"time"

	"github.com/moov-io/base/admin"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/util"

	"github.com/go-kit/kit/log"
)

func AddFileTransferSyncRoute(logger log.Logger, svc *admin.Server, flushIncoming FlushChan, flushOutgoing FlushChan) {
	svc.AddHandler("/files/flush/incoming", flushIncomingFiles(logger, flushIncoming))
	svc.AddHandler("/files/flush/outgoing", flushOutgoingFiles(logger, flushOutgoing))
	svc.AddHandler("/files/flush", flushFiles(logger, flushIncoming, flushOutgoing))

	svc.AddHandler("/files/merge", mergeOutgoingFiles(logger, flushOutgoing))
}

// flushIncomingFiles will download inbound and return files and then process them
func flushIncomingFiles(logger log.Logger, flushIncoming FlushChan) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		req := maybeWaiter(r)
		flushIncoming <- req
		if err := maybeWait(w, req); err == util.ErrTimeout {
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// flushOutgoingFiles will merge and upload outbound files
func flushOutgoingFiles(logger log.Logger, flushOutgoing FlushChan) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		req := maybeWaiter(r)
		flushOutgoing <- req
		if err := maybeWait(w, req); err == util.ErrTimeout {
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func flushFiles(logger log.Logger, flushIncoming FlushChan, flushOutgoing FlushChan) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		reqIncoming, reqOutgoing := maybeWaiter(r), maybeWaiter(r)
		flushIncoming <- reqIncoming
		flushOutgoing <- reqOutgoing
		if err := maybeWait(w, reqIncoming); err == util.ErrTimeout {
			return
		}
		if err := maybeWait(w, reqOutgoing); err == util.ErrTimeout {
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func mergeOutgoingFiles(logger log.Logger, outgoing FlushChan) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		req := maybeWaiter(r)
		req.skipUpload = true

		outgoing <- req
		if err := maybeWait(w, req); err == util.ErrTimeout {
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func maybeWaiter(r *http.Request) *periodicFileOperationsRequest {
	requestID, userID := moovhttp.GetRequestID(r), moovhttp.GetUserID(r)
	req := &periodicFileOperationsRequest{
		requestID: requestID,
		userID:    userID,
	}
	if _, exists := r.URL.Query()["wait"]; exists {
		req.waiter = make(chan struct{}, 1)
	}
	return req
}

func maybeWait(w http.ResponseWriter, req *periodicFileOperationsRequest) error {
	if req.waiter != nil {
		err := util.Timeout(func() error {
			<-req.waiter // wait for a response from StartPeriodicFileOperations
			return nil
		}, 30*time.Second)

		if err == util.ErrTimeout {
			moovhttp.Problem(w, err)
			return err
		}
	}
	return nil
}

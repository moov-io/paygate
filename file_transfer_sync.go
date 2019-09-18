// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package paygate

import (
	"fmt"
	"net/http"

	"github.com/moov-io/base/admin"
	moovhttp "github.com/moov-io/base/http"

	"github.com/go-kit/kit/log"
)

func AddFileTransferSyncRoute(logger log.Logger, svc *admin.Server, flushIncoming chan struct{}, flushOutgoing chan struct{}) {
	svc.AddHandler("/files/flush/incoming", flushIncomingFiles(logger, flushIncoming))
	svc.AddHandler("/files/flush/outgoing", flushOutgoingFiles(logger, flushOutgoing))
	svc.AddHandler("/files/flush", flushFiles(logger, flushIncoming, flushOutgoing))
}

// flushIncomingFiles will download inbound and return files and then process them
func flushIncomingFiles(logger log.Logger, flushIncoming chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		flushIncoming <- struct{}{} // send a message on the channel to trigger async routine

		w.WriteHeader(http.StatusOK)
	}
}

// flushOutgoingFiles will merge and upload outbound files
func flushOutgoingFiles(logger log.Logger, flushOutgoing chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		flushOutgoing <- struct{}{} // send a message on the channel to trigger async routine

		w.WriteHeader(http.StatusOK)
	}
}

func flushFiles(logger log.Logger, flushIncoming chan struct{}, flushOutgoing chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		flushIncoming <- struct{}{}
		flushOutgoing <- struct{}{}

		w.WriteHeader(http.StatusOK)
	}
}

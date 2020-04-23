// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"fmt"
	"net/http"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/util"

	"github.com/go-kit/kit/log"
)

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

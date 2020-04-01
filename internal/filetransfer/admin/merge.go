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

func mergeOutgoingFiles(logger log.Logger, outgoing FlushChan) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		req := maybeWaiter(r)
		req.SkipUpload = true

		outgoing <- req
		if err := maybeWait(w, req); err == util.ErrTimeout {
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

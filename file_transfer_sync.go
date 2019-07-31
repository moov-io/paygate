// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"

	"github.com/moov-io/base/admin"
	moovhttp "github.com/moov-io/base/http"

	"github.com/go-kit/kit/log"
)

func addFileTransferSyncRoute(logger log.Logger, svc *admin.Server, forceUpload chan struct{}) {
	svc.AddHandler("/files/upload", forceFileUpload(logger, forceUpload))
}

func forceFileUpload(logger log.Logger, forceUpload chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		forceUpload <- struct{}{} // send a messge for the receiving end

		w.WriteHeader(http.StatusOK)
	}
}

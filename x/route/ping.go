// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package route

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/moov-io/base/log"
)

func PingRoute(logger log.Logger, r *mux.Router) {
	r.Methods("GET").Path("/ping").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("PONG"))
	})
}

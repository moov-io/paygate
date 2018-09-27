// Copyright 2018 The ACH Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

// admin implements an http.Server which can be used for operations
// and monitoring tools. It's designed to be shipped (and ran) inside
// an existing Go service.
package admin

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"strings"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// profileEnabled returns if a given pprof handler should be
// enabled according to pprofHandlers and the PPROF_* environment
// variables.
//
// These profiles can be disabled by setting the appropriate PPROF_*
// environment variable. (i.e. PPROF_ALLOCS=no)
//
// An empty string, "yes", or "true" enables the profile. Any other
// value disables the profile.
func profileEnabled(name string) bool {
	k := fmt.Sprintf("PPROF_%s", strings.ToUpper(name))
	v := strings.ToLower(os.Getenv(k))
	return v == "" || v == "yes" || v == "true"
}

// Handler returns an http.Handler for the admin http service.
// This contains metrics and pprof handlers.
//
// No metrics specific to the handler are recorded.
//
// We only want to expose on the admin servlet because these
// profiles/dumps can contain sensitive info (raw memory).
func Handler() http.Handler {
	r := mux.NewRouter()

	// prometheus metrics
	r.Path("/metrics").Handler(promhttp.Handler())

	// always register index and cmdline handlers
	r.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	r.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))

	if profileEnabled("profile") {
		r.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	}
	if profileEnabled("symbol") {
		r.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	}
	if profileEnabled("trace") {
		r.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	}

	// Register runtime/pprof handlers
	if profileEnabled("allocs") {
		r.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	}
	if profileEnabled("block") {
		runtime.SetBlockProfileRate(1)
		r.Handle("/debug/pprof/block", pprof.Handler("block"))
	}
	if profileEnabled("goroutine") {
		r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	}
	if profileEnabled("heap") {
		r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	}
	if profileEnabled("mutex") {
		runtime.SetMutexProfileFraction(1)
		r.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	}
	if profileEnabled("threadcreate") {
		r.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	}

	return r
}

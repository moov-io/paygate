// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/base/idempotent"
	"github.com/moov-io/base/idempotent/lru"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/gorilla/mux"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const (
	// maxReadBytes is the number of bytes to read
	// from a request body. It's intended to be used
	// with an io.LimitReader
	maxReadBytes = 1 * 1024 * 1024
)

var (
	inmemIdempotentRecorder = lru.New()

	// Prometheus Metrics
	internalServerErrors = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Name: "http_errors",
		Help: "Count of how many 5xx errors we send out",
	}, nil)
	routeHistogram = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Name: "http_response_duration_seconds",
		Help: "Histogram representing the http response durations",
	}, []string{"route"})

	errMissingRequiredJson = errors.New("missing required JSON field(s)")
)

// read consumes an io.Reader (wrapping with io.LimitReader)
// and returns either the resulting bytes or a non-nil error.
func read(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, io.EOF
	}
	rr := io.LimitReader(r, maxReadBytes)
	return ioutil.ReadAll(rr)
}

func internalError(logger log.Logger, w http.ResponseWriter, err error) {
	internalServerErrors.Add(1)

	file := moovhttp.InternalError(w, err)
	component := strings.Split(file, ".go")[0]

	if logger != nil {
		logger.Log(component, err, "source", file)
	}
}

func addPingRoute(logger log.Logger, r *mux.Router) {
	r.Methods("GET").Path("/ping").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestId := moovhttp.GetRequestId(r); requestId != "" {
			logger.Log("route", "ping", "requestId", requestId)
		}
		moovhttp.SetAccessControlAllowHeaders(w, r.Header.Get("Origin"))
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("PONG"))
	})
}

func wrapResponseWriter(logger log.Logger, w http.ResponseWriter, r *http.Request) (http.ResponseWriter, error) {
	route := fmt.Sprintf("%s%s", strings.ToLower(r.Method), strings.Replace(r.URL.Path, "/", "-", -1)) // TODO(adam): filter out random ID's later

	writer, err := moovhttp.EnsureHeaders(logger, routeHistogram.With("route", route), inmemIdempotentRecorder, w, r)
	if err == idempotent.ErrSeenBefore {
		idempotent.SeenBefore(writer)
	}

	return writer, err
}

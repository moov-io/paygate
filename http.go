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
	"time"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/base/idempotent/lru"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// maxReadBytes is the number of bytes to read
	// from a request body. It's intended to be used
	// with an io.LimitReader
	maxReadBytes = 1 * 1024 * 1024
)

var (
	inmemIdempot = lru.New()

	errNoUserId = errors.New("no X-User-Id header provided")

	errMissingRequiredJson = errors.New("missing required JSON field(s)")

	pingResponseDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "response_duration_seconds",
			Help:        "A histogram of request latencies.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"app": "paygate", "route": "ping"},
		},
		[]string{"code"},
	)
)

// read consumes an io.Reader (wrapping with io.LimitReader)
// and returns either the resulting bytes or a non-nil error.
func read(r io.Reader) ([]byte, error) {
	r = io.LimitReader(r, maxReadBytes)
	return ioutil.ReadAll(r)
}

func internalError(w http.ResponseWriter, err error) {
	internalServerErrors.Add(1)

	file := moovhttp.InternalError(w, err)
	component := strings.Split(file, ".go")[0]

	logger.Log(component, err, "source", file)
}

func addPingRoute(r *mux.Router) {
	r.Methods("GET").Path("/ping").HandlerFunc(promhttp.InstrumentHandlerDuration(pingResponseDuration, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestId, userId := moovhttp.GetRequestId(r), moovhttp.GetUserId(r)
		if requestId != "" {
			if userId == "" {
				userId = "<none>"
			}
			logger.Log("route", "ping", "requestId", requestId, "userId", userId)
		}
		moovhttp.SetAccessControlAllowHeaders(w, r.Header.Get("Origin"))
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("PONG"))
	})))
}

// wrapResponseWriter creates a new paygateResponseWriter with sane values.
//
// Use the http.ResponseWriter as you normally would. When the status code is
// written then a log line and sample (metric) are recorded.
//
// method should be a unique name, i.e. http handler function name
func wrapResponseWriter(w http.ResponseWriter, r *http.Request, method string) (http.ResponseWriter, error) {
	ww := &paygateResponseWriter{
		ResponseWriter: w,
		start:          time.Now(),
		metric:         routeHistogram.With("route", method),
		method:         method,
		log:            logger,
	}
	if err := ww.ensureHeaders(r); err != nil {
		return ww, err
	}
	return ww, nil
}

// paygateResponseWriter embeds an http.ResponseWriter but also has knowledge
// of if headers have been written to emit a log line (with x-request-id) and
// record metrics.
//
// Use wrapResponseWriter to create a new instance, don't construct one yourself.
//
// This is not a thread-safe struct!
type paygateResponseWriter struct {
	http.ResponseWriter

	start  time.Time
	method string
	metric metrics.Histogram

	headersWritten    bool   // has .WriteHeader been called yet?
	userId, requestId string // X-Request-Id

	log log.Logger
}

// ensureHeaders verifies the headers which paygate cares about.
//  X-User-Id, X-Request-Id, and X-Idempotency-Key
//
// X-User-Id is required, and requests without one will be completed
// with a 403 forbidden.
//
// X-Request-Id is optional, but if used we will emit a log line with
// that request fulfillment timing and the status code.
//
// X-Idempotency-Key is optional, but recommended to ensure requests
// only execute once. Clients are assumed to resend requests many times
// with the same key. We just need to reply back "already done".
func (w *paygateResponseWriter) ensureHeaders(r *http.Request) error {
	if v := moovhttp.GetUserId(r); v == "" {
		if !w.headersWritten {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusForbidden)
		}
		return errNoUserId
	} else {
		w.userId = v
	}
	w.requestId = moovhttp.GetRequestId(r)
	return nil
}

// WriteHeader intercepts our usual
func (w *paygateResponseWriter) WriteHeader(code int) {
	if w.headersWritten {
		return
	}
	w.headersWritten = true
	defer w.ResponseWriter.WriteHeader(code)

	diff := time.Since(w.start)

	if w.metric != nil {
		w.metric.Observe(diff.Seconds())
	}

	if w.ResponseWriter.Header().Get("Content-Type") == "" {
		// skip Go's content sniff here to speed up rendering
		w.ResponseWriter.Header().Set("Content-Type", "text/plain")
	}

	if w.method != "" && w.requestId != "" {
		line := fmt.Sprintf("status=%d, took=%s, userId=%s, requestId=%s", code, diff, w.userId, w.requestId)
		w.log.Log(w.method, line)
	}
}

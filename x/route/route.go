// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package route

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/base/idempotent"
	"github.com/moov-io/base/idempotent/lru"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	IdempotentRecorder = lru.New()

	// Prometheus Metrics
	Histogram = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Name: "http_response_duration_seconds",
		Help: "Histogram representing the http response durations",
	}, []string{"route"})
)

// HeaderNamespace returns the namespace from HTTP Headers
func HeaderNamespace(r *http.Request) string {
	return r.Header.Get("X-Namespace")
}

type Responder struct {
	Namespace  string
	XRequestID string

	logger log.Logger

	request *http.Request
	span    opentracing.Span

	writer *moovhttp.ResponseWriter
}

func NewResponder(logger log.Logger, w http.ResponseWriter, r *http.Request) *Responder {
	resp := &Responder{
		Namespace:  HeaderNamespace(r),
		XRequestID: moovhttp.GetRequestID(r),
		logger:     logger,
		request:    r,
	}
	resp.setSpan()
	writer, err := wrapResponseWriter(logger, w, r)
	resp.writer = writer
	if err != nil {
		resp.Problem(err)
	}
	return resp
}

func (r *Responder) Log(kvpairs ...interface{}) {
	if r == nil || r.writer == nil {
		return
	}
	var args = []interface{}{
		"requestID", r.XRequestID,
		"namespace", r.Namespace,
	}
	for i := range kvpairs {
		args = append(args, kvpairs[i])
	}
	// TODO(adam): should we prefix args with the route info? e.g. /transfers/ is "transfers"
	r.logger.Log(args...)
}

func (r *Responder) Respond(fn func(http.ResponseWriter)) {
	if r == nil {
		return
	}
	// TODO(adam): we need to have a better framework for ensuring X-Namespace
	r.finishSpan()
	r.writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	fn(r.writer)
}

func (r *Responder) Problem(err error) {
	if r == nil {
		return
	}
	r.finishSpan()
	r.writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	moovhttp.Problem(r.writer, err)
}

func wrapResponseWriter(logger log.Logger, w http.ResponseWriter, r *http.Request) (*moovhttp.ResponseWriter, error) {
	name := fmt.Sprintf("%s-%s", strings.ToLower(r.Method), CleanPath(r.URL.Path))
	ww := moovhttp.Wrap(logger, Histogram.With("route", name), w, r)

	if _, seen := idempotent.FromRequest(r, IdempotentRecorder); seen {
		idempotent.SeenBefore(ww)
		return ww, idempotent.ErrSeenBefore
	}

	return ww, nil
}

var baseIdRegex = regexp.MustCompile(`([a-f0-9]{40})`)

// CleanPath takes a URL path and formats it for Prometheus metrics
//
// This method replaces /'s with -'s and strips out moov/base.ID() values from URL path slugs.
func CleanPath(path string) string {
	parts := strings.Split(path, "/")
	var out []string
	for i := range parts {
		if parts[i] == "" || baseIdRegex.MatchString(parts[i]) {
			continue // assume it's a moov/base.ID() value
		}
		out = append(out, parts[i])
	}
	return strings.Join(out, "-")
}

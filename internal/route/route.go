// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package route

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/base/idempotent/lru"
	"github.com/moov-io/paygate/pkg/id"

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

// GetUserID returns a wrapped UserID from an HTTP request
func GetUserID(r *http.Request) id.User {
	return id.User(moovhttp.GetUserID(r))
}

type Responder struct {
	XUserID    id.User
	XRequestID string

	logger log.Logger

	writer *moovhttp.ResponseWriter
}

func NewResponder(logger log.Logger, w http.ResponseWriter, r *http.Request) *Responder {
	writer, err := wrapResponseWriter(logger, w, r)
	if err != nil {
		return nil
	}
	return &Responder{
		XUserID:    GetUserID(r),
		XRequestID: moovhttp.GetRequestID(r),
		logger:     logger,
		writer:     writer,
	}
}

func (r *Responder) Log(kvpairs ...interface{}) {
	if r == nil || r.writer == nil {
		return
	}
	var args = []interface{}{
		"requestID", r.XRequestID,
		"userID", r.XUserID,
	}
	for i := range kvpairs {
		args = append(args, kvpairs[i])
	}
	r.logger.Log(args...)
}

func (r *Responder) Respond(fn func(http.ResponseWriter)) {
	if r == nil {
		return
	}
	r.writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	fn(r.writer)
}

func (r *Responder) Problem(err error) {
	if r == nil {
		return
	}
	r.writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	moovhttp.Problem(r.writer, err)
}

func wrapResponseWriter(logger log.Logger, w http.ResponseWriter, r *http.Request) (*moovhttp.ResponseWriter, error) {
	name := fmt.Sprintf("%s-%s", strings.ToLower(r.Method), CleanPath(r.URL.Path))
	return moovhttp.EnsureHeaders(logger, Histogram.With("route", name), IdempotentRecorder, w, r)
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

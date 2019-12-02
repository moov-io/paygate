// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/route"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

const (
	// maxReadBytes is the number of bytes to read
	// from a request body. It's intended to be used
	// with an io.LimitReader
	maxReadBytes = 1 * 1024 * 1024
)

var (
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

func AddPingRoute(logger log.Logger, r *mux.Router) {
	r.Methods("GET").Path("/ping").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestID := moovhttp.GetRequestID(r); requestID != "" {
			logger.Log("route", "ping", "requestID", requestID)
		}
		moovhttp.SetAccessControlAllowHeaders(w, r.Header.Get("Origin"))
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("PONG"))
	})
}

func Wrap(logger log.Logger, w http.ResponseWriter, r *http.Request) http.ResponseWriter {
	name := fmt.Sprintf("%s-%s", strings.ToLower(r.Method), route.CleanPath(r.URL.Path))
	return moovhttp.Wrap(logger, route.Histogram.With("route", name), w, r)
}

func TLSHttpClient(path string) (*http.Client, error) {
	tlsConfig := &tls.Config{}
	pool, err := x509.SystemCertPool()
	if pool == nil || err != nil {
		pool = x509.NewCertPool()
	}

	// read extra CA file
	if path != "" {
		bs, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("problem reading %s: %v", path, err)
		}
		ok := pool.AppendCertsFromPEM(bs)
		if !ok {
			return nil, fmt.Errorf("couldn't parse PEM in: %s", path)
		}
	}
	tlsConfig.RootCAs = pool

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     tlsConfig,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			MaxConnsPerHost:     100,
			IdleConnTimeout:     1 * time.Minute,
		},
	}, nil
}

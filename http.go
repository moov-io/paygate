// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
)

const (
	// maxReadBytes is the number of bytes to read
	// from a request body. It's intended to be used
	// with an io.LimitReader
	maxReadBytes = 1 * 1024 * 1024
)

// read consumes an io.Reader (wrapping with io.LimitReader)
// and returns either the resulting bytes or a non-nil error.
func read(r io.Reader) ([]byte, error) {
	r = io.LimitReader(r, maxReadBytes)
	return ioutil.ReadAll(r)
}

// getUserId grabs the userId from the http header, which is
// trusted. (The infra ensures this)
func getUserId(r *http.Request) string {
	return r.Header.Get("X-User-Id")
}

// getIdempotencyKey extracts X-Idempotency-Key from the http request,
// which is used to ensure transactions only commit once.
func getIdempotencyKey(r *http.Request) string {
	if v := r.Header.Get("X-Idempotency-Key"); v != "" {
		return v
	}
	return nextID()
}

// getRequestId extracts X-Request-Id from the http request, which
// is used in tracing requests.
func getRequestId(r *http.Request) string {
	if v := r.Header.Get("X-Request-Id"); v != "" {
		return v
	}
	return nextID()
}

// encodeError JSON encodes the supplied error
//
// The HTTP status of "400 Bad Request" is written to the
// response.
func encodeError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

func internalError(w http.ResponseWriter, err error, component string) {
	internalServerErrors.Add(1)
	logger.Log(component, err)
	w.WriteHeader(http.StatusInternalServerError)
}

func addPingRoute(r *mux.Router) {
	r.Methods("GET").Path("/ping").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")

		userId := getUserId(r)
		if userId == "" {
			w.Write([]byte("PONG"))
		} else {
			w.Write([]byte(fmt.Sprintf("hello %s", userId)))
		}
	})
}

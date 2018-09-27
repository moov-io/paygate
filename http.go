// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
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

// TODO(adam)
// Idempotency-Key
// Request-Id

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
		w.Write([]byte("PONG"))
	})
}

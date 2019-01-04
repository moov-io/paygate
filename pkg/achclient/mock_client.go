// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

var (
	AddPingRoute = func(r *mux.Router) {
		r.Methods("GET").Path("/ping").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("PONG"))
			w.WriteHeader(http.StatusOK)
		})
	}

	AddCreateRoute = func(ww *httptest.ResponseRecorder, r *mux.Router) {
		r.Methods("POST").Path("/files/create").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set response headers
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if v := r.Header.Get("X-Idempotency-Key"); v != "" {
				// copy header to response (for tests)
				w.Header().Set("X-Idempotency-Key", v)
			}

			type response struct {
				ID string `json:"id"` // ach.File ID
			}

			bs, _ := ioutil.ReadAll(r.Body)

			var resp response
			if ww != nil && len(bs) != 0 {
				// write incoming body to our test ResponseRecorder
				n, err := io.Copy(ww, bytes.NewReader(bs))
				if err != nil || n == 0 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			// Grab request body for use in response
			if err := json.NewDecoder(bytes.NewReader(bs)).Decode(&resp); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(`{"id": "%s", "error": null}`, resp.ID)))
		})
	}

	AddValidateRoute = func(r *mux.Router) {
		r.Methods("GET").Path("/files/{fileId}/validate").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"error":null}`))
		})
	}

	AddDeleteRoute = func(r *mux.Router) {
		r.Methods("DELETE").Path("/files/delete").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
		})
	}
)

func MockClientServer(name string, routes ...func(*mux.Router)) (*ACH, *http.Client, *httptest.Server) {
	r := mux.NewRouter()
	for i := range routes {
		routes[i](r) // Add each route
	}
	server := httptest.NewServer(r)
	client := server.Client()

	achClient := New(name, log.NewNopLogger())
	achClient.client = client
	achClient.endpoint = server.URL

	return achClient, client, server
}

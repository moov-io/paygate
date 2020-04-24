// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package testclient

import (
	"net/http/httptest"
	"testing"

	"github.com/moov-io/paygate/client"

	"github.com/gorilla/mux"
)

func New(t *testing.T, handler *mux.Router) *client.APIClient {
	server := httptest.NewServer(handler)
	t.Cleanup(func() { server.Close() })

	conf := client.NewConfiguration()
	conf.BasePath = server.URL

	return client.NewAPIClient(conf)
}

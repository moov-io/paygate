// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"context"
	"testing"

	"github.com/moov-io/paygate/internal/testclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestRouter__getUserTenants(t *testing.T) {
	r := mux.NewRouter()
	router := NewRouter(log.NewNopLogger(), &mockRepository{})
	router.RegisterRoutes(r)

	client := testclient.New(t, r)

	ts, resp, err := client.TenantsApi.GetTenants(context.TODO(), "userID", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if n := len(ts); n != 1 {
		t.Errorf("got %d tenants: %#v", n, ts)
	}
}

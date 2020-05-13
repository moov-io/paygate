// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"context"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/testclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestRouter__getUserTenants(t *testing.T) {
	repo := &MockRepository{
		Tenants: []client.Tenant{
			{
				TenantID:        base.ID(),
				Name:            "My Company",
				PrimaryCustomer: base.ID(),
			},
		},
	}

	r := mux.NewRouter()
	router := NewRouter(log.NewNopLogger(), repo)
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

func TestRouter__UpdateTenant(t *testing.T) {
	tenantID := base.ID()
	repo := &MockRepository{
		Tenants: []client.Tenant{
			{
				TenantID:        tenantID,
				Name:            "My Company",
				PrimaryCustomer: base.ID(),
			},
		},
	}

	r := mux.NewRouter()
	router := NewRouter(log.NewNopLogger(), repo)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	req := client.UpdateTenant{
		Name: "New Name",
	}
	resp, err := c.TenantsApi.UpdateTenant(context.TODO(), tenantID, "userID", req, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

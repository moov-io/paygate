// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/admin"
	"github.com/moov-io/paygate/pkg/tenants"
	"github.com/moov-io/paygate/pkg/testclient"
)

func TestRoutes__Create(t *testing.T) {
	repo := &tenants.MockRepository{}

	svc, c := testclient.Admin(t)
	RegisterRoutes(log.NewNopLogger(), svc, repo)

	req := admin.CreateTenant{
		Name:            "My Company",
		PrimaryCustomer: base.ID(),
	}

	userID := base.ID()
	tenant, resp, err := c.TenantsApi.CreateTenant(context.Background(), userID, req, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}

	if tenant.Name != req.Name {
		t.Errorf("tenant.Name=%q", tenant.Name)
	}
}

func TestRoutes__CreateErr(t *testing.T) {
	repo := &tenants.MockRepository{
		Err: errors.New("bad error"),
	}

	svc, c := testclient.Admin(t)
	RegisterRoutes(log.NewNopLogger(), svc, repo)

	req := admin.CreateTenant{
		Name:            "My Company",
		PrimaryCustomer: base.ID(),
	}

	userID := base.ID()
	_, resp, err := c.TenantsApi.CreateTenant(context.Background(), userID, req, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error")
	}

	// invalid request body
	repo.Err = nil
	req.Name = ""

	_, resp, err = c.TenantsApi.CreateTenant(context.Background(), userID, req, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

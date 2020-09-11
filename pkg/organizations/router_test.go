// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organizations

import (
	"context"
	"errors"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/testclient"

	"github.com/antihax/optional"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestRouter__getOrganizations(t *testing.T) {
	repo := &mockRepository{
		Organizations: []client.Organization{
			{
				OrganizationID:  base.ID(),
				Name:            "my organization",
				TenantID:        base.ID(),
				PrimaryCustomer: base.ID(),
			},
		},
	}

	r := mux.NewRouter()
	router := NewRouter(log.NewNopLogger(), repo)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	opts := &client.GetOrganizationsOpts{
		XRequestID: optional.NewString("req"),
	}
	orgs, resp, err := c.OrganizationsApi.GetOrganizations(context.TODO(), "tenantID", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if n := len(orgs); n != 1 {
		t.Errorf("got %d organizations: %#v", n, orgs)
	}
}

func TestRouter__getOrganizationsErr(t *testing.T) {
	repo := &mockRepository{
		Err: errors.New("bad error"),
	}
	r := mux.NewRouter()
	router := NewRouter(log.NewNopLogger(), repo)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	opts := &client.GetOrganizationsOpts{
		XRequestID: optional.NewString("req"),
	}
	orgs, resp, err := c.OrganizationsApi.GetOrganizations(context.TODO(), "tenantID", opts)
	defer resp.Body.Close()

	if err == nil {
		t.Error("expected error")
	}
	if len(orgs) != 0 {
		t.Errorf("unexpcted organizations: %#v", orgs)
	}
}

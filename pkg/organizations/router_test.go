// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organizations

import (
	"context"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/testclient"
	"github.com/moov-io/paygate/pkg/client"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestRouter__getUserTenants(t *testing.T) {
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

	client := testclient.New(t, r)

	orgs, resp, err := client.OrganizationsApi.GetOrganizations(context.TODO(), "userID", "tenantID", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if n := len(orgs); n != 1 {
		t.Errorf("got %d organizations: %#v", n, orgs)
	}
}

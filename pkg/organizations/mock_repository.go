// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organizations

import (
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/id"
)

type mockRepository struct {
	Organizations []client.Organization
	Err           error
}

func (r *mockRepository) getOrganizations(userID id.User) ([]client.Organization, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Organizations, nil
}

func (r *mockRepository) createOrganization(userID id.User, org client.Organization) error {
	return r.Err
}

func (r *mockRepository) updateOrganizationName(orgID, name string) error {
	return r.Err
}

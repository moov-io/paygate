// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package tenants

import (
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/database"
)

func setupSQLiteDB(t *testing.T) *sqlRepo {
	db := database.CreateTestSqliteDB(t)
	t.Cleanup(func() { db.Close() })

	repo := &sqlRepo{db: db.DB}
	t.Cleanup(func() { repo.Close() })

	return repo
}

func setupMySQLeDB(t *testing.T) *sqlRepo {
	db := database.CreateTestMySQLDB(t)
	t.Cleanup(func() { db.Close() })

	repo := &sqlRepo{db: db.DB}
	t.Cleanup(func() { repo.Close() })

	return repo
}

func writeTenant(t *testing.T, tenantID string, repo Repository) client.Tenant {
	t.Helper()

	tenant := client.Tenant{
		TenantID:        tenantID,
		Name:            "My Company",
		PrimaryCustomer: base.ID(),
	}
	if err := repo.Create(tenant, "companyID"); err != nil {
		t.Fatal(err)
	}
	return tenant
}

func TestRepository__Create(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		tenantID := base.ID()
		tenant := writeTenant(t, tenantID, repo)
		if tenant.TenantID == "" {
			t.Errorf("missing tenant: %#v", tenant)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

func TestRepository__List(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		tenantID := base.ID()
		tenant := writeTenant(t, tenantID, repo)

		tenants, err := repo.List(tenantID)
		if err != nil {
			t.Fatal(err)
		}
		if len(tenants) == 0 || tenants[0].TenantID != tenant.TenantID {
			t.Errorf("unexpected Tenants %#v", tenants)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

func TestRepository__GetCompanyIdentification(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		tenantID := base.ID()
		tenant := writeTenant(t, tenantID, repo)

		companyID, err := repo.GetCompanyIdentification(tenant.TenantID)
		if err != nil {
			t.Fatal(err)
		}
		if companyID != "companyID" {
			t.Errorf("unexpected companyID=%q", companyID)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

func TestRepository__UpdateTenant(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		tenantID := base.ID()
		tenant := writeTenant(t, tenantID, repo)

		req := client.UpdateTenant{
			Name: base.ID(),
		}
		if err := repo.UpdateTenant(tenant.TenantID, req); err != nil {
			t.Fatal(err)
		}

		tenants, err := repo.List(tenantID)
		if err != nil {
			t.Fatal(err)
		}
		if len(tenants) != 1 {
			t.Errorf("unexpected tenants: %#v", tenants)
		}

		if tenants[0].Name != req.Name {
			t.Errorf("improper update: %#v", tenants[0])
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

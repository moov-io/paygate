// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organization

import (
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/base/database"
)

func setupSQLiteDB(t *testing.T) *sqlRepo {
	db := database.CreateTestSQLiteDB(t)
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

func writeConfig(t *testing.T, orgID string, cfg Config, repo *sqlRepo) {
	t.Helper()

	query := `insert into organization_configs (organization, company_identification) values (?, ?);`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stmt.Exec(orgID, cfg.CompanyIdentification)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRepository__GetConfig(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		orgID := base.ID()

		if cfg, err := repo.GetConfig(orgID); cfg != nil || err != nil {
			t.Fatalf("cfg=%#v  error=%v", cfg, err)
		}

		// write config
		writeConfig(t, orgID, Config{CompanyIdentification: "foo"}, repo)

		cfg, err := repo.GetConfig(orgID)
		if err != nil {
			t.Fatal(err)
		}
		if cfg == nil {
			t.Fatal("nil Config")
		}
		if cfg.CompanyIdentification != "foo" {
			t.Fatalf("CompanyIdentification=%q", cfg.CompanyIdentification)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

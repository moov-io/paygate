// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"database/sql"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/database"
)

func TestRepository__getMicroDeposits(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		micro := writeMicroDeposits(t, repo)
		micro, err := repo.getMicroDeposits(micro.MicroDepositID)
		if err != nil {
			t.Fatal(err)
		}
		if micro == nil {
			t.Error("missing MicroDeposit")
		}

		micro, err = repo.getMicroDeposits(base.ID())
		if err != sql.ErrNoRows {
			t.Error(err)
		}
		if micro != nil {
			t.Errorf("unexpected micro-deposit: %v", micro)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

func TestRepository__getAccountMicroDeposits(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqlRepo) {
		micro := writeMicroDeposits(t, repo)
		micro, err := repo.getAccountMicroDeposits(micro.Destination.AccountID)
		if err != nil {
			t.Fatal(err)
		}
		if micro == nil {
			t.Error("missing MicroDeposit")
		}

		micro, err = repo.getAccountMicroDeposits(base.ID())
		if err != sql.ErrNoRows {
			t.Error(err)
		}
		if micro != nil {
			t.Errorf("unexpected micro-deposit: %v", micro)
		}
	}

	check(t, setupSQLiteDB(t))
	check(t, setupMySQLeDB(t))
}

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

func writeMicroDeposits(t *testing.T, repo Repository) *client.MicroDeposits {
	t.Helper()

	micro := &client.MicroDeposits{
		MicroDepositID: base.ID(),
		TransferIDs:    []string{base.ID(), base.ID()},
		Destination: client.Destination{
			CustomerID: base.ID(),
			AccountID:  base.ID(),
		},
		Amounts: []client.Amount{
			{Currency: "USD", Value: 2},
			{Currency: "USD", Value: 5},
		},
		Status:  client.PENDING,
		Created: time.Now(),
	}
	if err := repo.writeMicroDeposits(micro); err != nil {
		t.Fatal(err)
	}
	return micro
}

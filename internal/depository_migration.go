// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"time"

	"github.com/moov-io/paygate/internal/secrets"

	"github.com/go-kit/kit/log"
)

func EncryptStoredAccountNumbers(logger log.Logger, repo *SQLDepositoryRepo, keeper *secrets.StringKeeper) error {
	var max time.Time
	for {
		rows, err := grabEncryptableDepositories(logger, repo, max, 100)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil // done
		} else {
			max = rows[len(rows)-1].createdAt // update our next starting point
		}

		for i := range rows {
			dep, err := repo.GetDepository(DepositoryID(rows[i].id))
			if err != nil {
				return err
			}

			dep.AccountNumber, err = keeper.EncryptString(rows[i].accountNumber)
			if err != nil {
				return err
			}

			if err := repo.UpsertUserDepository(dep.UserID(), dep); err != nil {
				return err
			}
		}
	}

	return nil
}

type encryptableDepository struct {
	id            string
	accountNumber string
	createdAt     time.Time
}

func grabEncryptableDepositories(logger log.Logger, repo *SQLDepositoryRepo, newerThan time.Time, batchSize int) ([]encryptableDepository, error) {
	query := `select depository_id, account_number, created_at from depositories
where account_number <> '' and account_number_encrypted = '' and created_at > ?
order by created_at asc limit ?;`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query(newerThan, batchSize)
	if err != nil {
		return nil, err
	}

	var out []encryptableDepository
	for rows.Next() {
		var row encryptableDepository
		if err := rows.Scan(&row.id, &row.accountNumber, &row.createdAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

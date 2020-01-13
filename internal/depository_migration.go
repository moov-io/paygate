// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

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
			return nil // no more records, so done
		} else {
			max = rows[len(rows)-1].createdAt // update our next starting point
		}
		for i := range rows {
			dep, err := repo.GetDepository(id.Depository(rows[i].id))
			if err != nil {
				return err
			}

			dep.EncryptedAccountNumber, err = keeper.EncryptString(rows[i].accountNumber)
			if err != nil {
				return err
			}
			if hash, err := hashAccountNumber(rows[i].accountNumber); err != nil {
				return err
			} else {
				dep.hashedAccountNumber = hash
			}

			if err := repo.UpsertUserDepository(id.User(dep.UserID()), dep); err != nil {
				return err
			}
		}
	}
}

func hashAccountNumber(num string) (string, error) {
	ss := sha256.New()
	n, err := ss.Write([]byte(num))
	if n == 0 || err != nil {
		return "", fmt.Errorf("sha256: n=%d: %v", n, err)
	}
	return hex.EncodeToString(ss.Sum(nil)), nil
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
	defer stmt.Close()

	rows, err := stmt.Query(newerThan, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

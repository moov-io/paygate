// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package returns

import (
	"database/sql"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

// FromMicroDeposits hooks into the micro_deposits table to grab any return_codes
// we received from return files after sending off the initial credits.
//
// This isn't ideal as we'd like to separate the concerns of micro_deposits from a Depository.
func FromMicroDeposits(db *sql.DB, id id.Depository) []*ach.ReturnCode {
	query := `select distinct md.return_code from micro_deposits as md
inner join depositories as deps on md.depository_id = deps.depository_id
where md.depository_id = ? and deps.status = ? and md.return_code <> '' and md.deleted_at is null and deps.deleted_at is null`
	stmt, err := db.Prepare(query)
	if err != nil {
		return nil
	}
	defer stmt.Close()

	rows, err := stmt.Query(id, model.DepositoryRejected)
	if err != nil {
		return nil
	}
	defer rows.Close()

	returnCodes := make(map[string]*ach.ReturnCode)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil
		}
		if _, exists := returnCodes[code]; !exists {
			returnCodes[code] = ach.LookupReturnCode(code)
		}
	}

	var codes []*ach.ReturnCode
	for k := range returnCodes {
		codes = append(codes, returnCodes[k])
	}
	return codes
}

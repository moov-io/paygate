// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package customers

import (
	"database/sql"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
)

func TestCursor(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	cur := NewCursor(log.NewNopLogger(), db.DB, 2)
	defer cur.Close()

	customers, err := cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(customers) != 0 {
		t.Errorf("customers=%#v", customers)
	}

	// write an originator and receiver
	origID, recID := base.ID(), base.ID()
	customerID := base.ID()

	if err := writeOriginator(db.DB, origID, customerID); err != nil {
		t.Fatal(err)
	}
	if err := writeReceiver(db.DB, recID, customerID); err != nil {
		t.Fatal(err)
	}

	customers, err = cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(customers) != 2 {
		t.Errorf("customers=%#v", customers)
	}
	for i := range customers {
		if customers[i].OriginatorID == origID {
			continue
		}
		if customers[i].ReceiverID == recID {
			continue
		}
		t.Errorf("unexpected customer: %#v", customers[i])
	}

	// call again and get nothing
	customers, err = cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(customers) != 0 {
		t.Errorf("customers=%#v", customers)
	}
}

func writeOriginator(db *sql.DB, id, customerID string) error {
	query := `insert into originators (originator_id, customer_id, created_at) values (?, ?, ?)`
	stmt, err := db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(id, customerID, time.Now())
	return err
}

func writeReceiver(db *sql.DB, id, customerID string) error {
	query := `insert into receivers (receiver_id, customer_id, created_at) values (?, ?, ?)`
	stmt, err := db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(id, customerID, time.Now())
	return err
}

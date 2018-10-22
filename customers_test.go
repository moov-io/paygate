// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
)

func TestCustomerStatus__json(t *testing.T) {
	cs := CustomerStatus("invalid")
	valid := map[string]CustomerStatus{
		"unverified":  CustomerUnverified,
		"verIFIed":    CustomerVerified,
		"SUSPENDED":   CustomerSuspended,
		"deactivated": CustomerDeactivated,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &cs); err != nil {
			t.Error(err.Error())
		}
		if cs != v {
			t.Errorf("got cs=%#v, v=%#v", cs, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, nextID()))
	if err := json.Unmarshal(in, &cs); err == nil {
		t.Error("expected error")
	}
}

func TestCustomers__emptyDB(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteCustomerRepo{
		db:  db.db,
		log: log.NewNopLogger(),
	}

	userId := nextID()
	if err := r.deleteUserCustomer(CustomerID(nextID()), userId); err != nil {
		t.Errorf("expected no error, but got %v", err)
	}

	// all customers for a user
	customers, err := r.getUserCustomers(userId)
	if err != nil {
		t.Error(err)
	}
	if len(customers) != 0 {
		t.Errorf("expected empty, got %v", customers)
	}

	// specific customer
	cust, err := r.getUserCustomer(CustomerID(nextID()), userId)
	if err != nil {
		t.Error(err)
	}
	if cust != nil {
		t.Errorf("expected empty, got %v", cust)
	}
}

func TestCustomers__upsert(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteCustomerRepo{db.db, log.NewNopLogger()}
	userId := nextID()

	cust := &Customer{
		ID:                CustomerID(nextID()),
		Email:             "test@moov.io",
		DefaultDepository: DepositoryID(nextID()),
		Status:            CustomerVerified,
		Metadata:          "extra data",
		Created:           time.Now(),
	}
	if c, err := r.getUserCustomer(cust.ID, userId); err != nil || c != nil {
		t.Errorf("expected empty, c=%v | err=%v", c, err)
	}

	// write, then verify
	if err := r.upsertUserCustomer(userId, cust); err != nil {
		t.Error(err)
	}

	c, err := r.getUserCustomer(cust.ID, userId)
	if err != nil {
		t.Error(err)
	}
	if c.ID != cust.ID {
		t.Errorf("c.ID=%q, cust.ID=%q", c.ID, cust.ID)
	}
	if c.Email != cust.Email {
		t.Errorf("c.Email=%q, cust.Email=%q", c.Email, cust.Email)
	}
	if c.DefaultDepository != cust.DefaultDepository {
		t.Errorf("c.DefaultDepository=%q, cust.DefaultDepository=%q", c.DefaultDepository, cust.DefaultDepository)
	}
	if c.Status != cust.Status {
		t.Errorf("c.Status=%q, cust.Status=%q", c.Status, cust.Status)
	}
	if c.Metadata != cust.Metadata {
		t.Errorf("c.Metadata=%q, cust.Metadata=%q", c.Metadata, cust.Metadata)
	}
	if !c.Created.Equal(cust.Created) {
		t.Errorf("c.Created=%q, cust.Created=%q", c.Created, cust.Created)
	}

	// get all for our user
	customers, err := r.getUserCustomers(userId)
	if err != nil {
		t.Error(err)
	}
	if len(customers) != 1 {
		t.Errorf("expected one, got %v", customers)
	}
	if customers[0].ID != cust.ID {
		t.Errorf("customers[0].ID=%q, cust.ID=%q", customers[0].ID, cust.ID)
	}

	// update, verify default depository changed
	depositoryId := DepositoryID(nextID())
	cust.DefaultDepository = depositoryId
	if err := r.upsertUserCustomer(userId, cust); err != nil {
		t.Error(err)
	}
	if cust.DefaultDepository != depositoryId {
		t.Errorf("got %q", cust.DefaultDepository)
	}
}

func TestCustomers__delete(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteCustomerRepo{db.db, log.NewNopLogger()}
	userId := nextID()

	cust := &Customer{
		ID:                CustomerID(nextID()),
		Email:             "test@moov.io",
		DefaultDepository: DepositoryID(nextID()),
		Status:            CustomerVerified,
		Metadata:          "extra data",
		Created:           time.Now(),
	}
	if c, err := r.getUserCustomer(cust.ID, userId); err != nil || c != nil {
		t.Errorf("expected empty, c=%v | err=%v", c, err)
	}

	// write
	if err := r.upsertUserCustomer(userId, cust); err != nil {
		t.Error(err)
	}

	// verify
	c, err := r.getUserCustomer(cust.ID, userId)
	if err != nil || c == nil {
		t.Errorf("expected customer, c=%v, err=%v", c, err)
	}

	// delete
	if err := r.deleteUserCustomer(cust.ID, userId); err != nil {
		t.Error(err)
	}

	// verify tombstoned
	if c, err := r.getUserCustomer(cust.ID, userId); err != nil || c != nil {
		t.Errorf("expected empty, c=%v | err=%v", c, err)
	}
}

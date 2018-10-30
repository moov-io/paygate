// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
)

func TestDepositories__depositoryRequest(t *testing.T) {
	req := depositoryRequest{}
	if !req.missingFields() {
		t.Error("expected error")
	}
}

func TestDepository__types(t *testing.T) {
	if !DepositoryStatus("").empty() {
		t.Error("expected empty")
	}
}

func TestDepositoriesHolderType__json(t *testing.T) {
	ht := HolderType("invalid")
	valid := map[string]HolderType{
		"indIVIdual": Individual,
		"Business":   Business,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &ht); err != nil {
			t.Error(err.Error())
		}
		if ht != v {
			t.Errorf("got ht=%#v, v=%#v", ht, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, nextID()))
	if err := json.Unmarshal(in, &ht); err == nil {
		t.Error("expected error")
	}
}

func TestDepositories__read(t *testing.T) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(depositoryRequest{
		BankName:      "test",
		Holder:        "me",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123456789",
		AccountNumber: "123",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := readDepositoryRequest(&http.Request{
		Body: ioutil.NopCloser(&buf),
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.BankName != "test" {
		t.Error(req.BankName)
	}
	if req.Holder != "me" {
		t.Error(req.Holder)
	}
	if req.HolderType != Individual {
		t.Error(req.HolderType)
	}
	if req.Type != Checking {
		t.Error(req.Type)
	}
	if req.RoutingNumber != "123456789" {
		t.Error(req.RoutingNumber)
	}
	if req.AccountNumber != "123" {
		t.Error(req.AccountNumber)
	}
}

func TestDepositorStatus__json(t *testing.T) {
	ht := DepositoryStatus("invalid")
	valid := map[string]DepositoryStatus{
		"Verified":   DepositoryVerified,
		"unverifieD": DepositoryUnverified,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &ht); err != nil {
			t.Error(err.Error())
		}
		if ht != v {
			t.Errorf("got ht=%#v, v=%#v", ht, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, nextID()))
	if err := json.Unmarshal(in, &ht); err == nil {
		t.Error("expected error")
	}
}

func TestDepositories__emptyDB(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteDepositoryRepo{
		db:  db.db,
		log: log.NewNopLogger(),
	}

	userId := nextID()
	if err := r.deleteUserDepository(DepositoryID(nextID()), userId); err != nil {
		t.Errorf("expected no error, but got %v", err)
	}

	// all customers for a user
	customers, err := r.getUserDepositories(userId)
	if err != nil {
		t.Error(err)
	}
	if len(customers) != 0 {
		t.Errorf("expected empty, got %v", customers)
	}

	// specific customer
	cust, err := r.getUserDepository(DepositoryID(nextID()), userId)
	if err != nil {
		t.Error(err)
	}
	if cust != nil {
		t.Errorf("expected empty, got %v", cust)
	}

	// depository check
	if depositoryIdExists(userId, DepositoryID(nextID()), r) {
		t.Error("DepositoryId shouldn't exist")
	}
}

func TestDepositories__upsert(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteDepositoryRepo{db.db, log.NewNopLogger()}
	userId := nextID()

	dep := &Depository{
		ID:            DepositoryID(nextID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       time.Now().Add(-1 * time.Second),
	}
	if d, err := r.getUserDepository(dep.ID, userId); err != nil || d != nil {
		t.Errorf("expected empty, d=%v | err=%v", d, err)
	}

	// write, then verify
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Error(err)
	}

	d, err := r.getUserDepository(dep.ID, userId)
	if err != nil {
		t.Error(err)
	}
	if d == nil {
		t.Fatal("expected Depository, got nil")
	}
	if d.ID != dep.ID {
		t.Errorf("d.ID=%q, dep.ID=%q", d.ID, dep.ID)
	}

	// get all for our user
	depositories, err := r.getUserDepositories(userId)
	if err != nil {
		t.Error(err)
	}
	if len(depositories) != 1 {
		t.Errorf("expected one, got %v", depositories)
	}
	if depositories[0].ID != dep.ID {
		t.Errorf("depositories[0].ID=%q, dep.ID=%q", depositories[0].ID, dep.ID)
	}

	// update, verify default depository changed
	bankName := "my new bank"
	dep.BankName = bankName
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Error(err)
	}
	d, err = r.getUserDepository(dep.ID, userId)
	if err != nil {
		t.Error(err)
	}
	if dep.BankName != d.BankName {
		t.Errorf("got %q", d.BankName)
	}

	if !depositoryIdExists(userId, dep.ID, r) {
		t.Error("DepositoryId should exist")
	}
}

func TestDepositories__delete(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteDepositoryRepo{db.db, log.NewNopLogger()}
	userId := nextID()

	dep := &Depository{
		ID:            DepositoryID(nextID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       time.Now().Add(-1 * time.Second),
	}
	if d, err := r.getUserDepository(dep.ID, userId); err != nil || d != nil {
		t.Errorf("expected empty, d=%v | err=%v", d, err)
	}

	// write
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Error(err)
	}

	// verify
	d, err := r.getUserDepository(dep.ID, userId)
	if err != nil || d == nil {
		t.Errorf("expected depository, d=%v, err=%v", d, err)
	}

	// delete
	if err := r.deleteUserDepository(dep.ID, userId); err != nil {
		t.Error(err)
	}

	// verify tombstoned
	if d, err := r.getUserDepository(dep.ID, userId); err != nil || d != nil {
		t.Errorf("expected empty, d=%v | err=%v", d, err)
	}

	if depositoryIdExists(userId, dep.ID, r) {
		t.Error("DepositoryId shouldn't exist")
	}
}

func TestDepositories__approved(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	r := &sqliteDepositoryRepo{db.db, log.NewNopLogger()}
	userId := nextID()

	dep := &Depository{
		ID:            DepositoryID(nextID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryVerified,
		Created:       time.Now().Add(-1 * time.Second),
	}
	if err := r.upsertUserDepository(userId, dep); err != nil {
		t.Error(err)
	}

	// Check
	if !depositoryApproved(userId, dep.ID, r) {
		t.Error("expected Depository approved")
	}
}

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
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestTransfers__transferRequest(t *testing.T) {
	req := transferRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestTransferType__json(t *testing.T) {
	tt := TransferType("invalid")
	valid := map[string]TransferType{
		"Pull": PullTransfer,
		"push": PushTransfer,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &tt); err != nil {
			t.Error(err.Error())
		}
		if tt != v {
			t.Errorf("got tt=%#v, v=%#v", tt, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, nextID()))
	if err := json.Unmarshal(in, &tt); err == nil {
		t.Error("expected error")
	}
}

func TestTransferStatus__json(t *testing.T) {
	ts := TransferStatus("invalid")
	valid := map[string]TransferStatus{
		"Canceled":  TransferCanceled,
		"Failed":    TransferFailed,
		"PENDING":   TransferPending,
		"Processed": TransferProcessed,
		"reclaimed": TransferReclaimed,
	}
	for k, v := range valid {
		in := []byte(fmt.Sprintf(`"%v"`, k))
		if err := json.Unmarshal(in, &ts); err != nil {
			t.Error(err.Error())
		}
		if !ts.Equal(v) {
			t.Errorf("got ts=%#v, v=%#v", ts, v)
		}
	}

	// make sure other values fail
	in := []byte(fmt.Sprintf(`"%v"`, nextID()))
	if err := json.Unmarshal(in, &ts); err == nil {
		t.Error("expected error")
	}
}

func TestTransfers__read(t *testing.T) {
	var buf bytes.Buffer
	amt, _ := NewAmount("USD", "27.12")
	err := json.NewEncoder(&buf).Encode(transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Customer:               CustomerID("customer"),
		CustomerDepository:     DepositoryID("customer"),
		Description:            "paycheck",
		StandardEntryClassCode: "PPD",
	})
	if err != nil {
		t.Fatal(err)
	}
	requests, err := readTransferRequests(&http.Request{
		Body: ioutil.NopCloser(&buf),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Error(requests)
	}
	req := requests[0]
	if req.Type != PushTransfer {
		t.Error(req.Type)
	}
	if v := req.Amount.String(); v != "USD 27.12" {
		t.Error(v)
	}
	if req.Originator != "originator" {
		t.Error(req.Originator)
	}
	if req.OriginatorDepository != "originator" {
		t.Error(req.OriginatorDepository)
	}
	if req.Customer != "customer" {
		t.Error(req.Customer)
	}
	if req.CustomerDepository != "customer" {
		t.Error(req.CustomerDepository)
	}
	if req.Description != "paycheck" {
		t.Error(req.Description)
	}
	if req.StandardEntryClassCode != "PPD" {
		t.Error(req.StandardEntryClassCode)
	}
}

func TestTransfers__idempotency(t *testing.T) {
	r := mux.NewRouter()
	// The repositories aren't used, aka idempotency check needs to be first.
	addTransfersRoute(r, nil, nil, nil, nil, nil)

	server := httptest.NewServer(r)
	client := server.Client()

	req, _ := http.NewRequest("POST", server.URL+"/transfers", nil)
	req.Header.Set("X-Idempotency-Key", "key")
	req.Header.Set("X-User-Id", "user")

	// mark the key as seen
	inmemIdempot.SeenBefore("key")

	// make our request
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("got %d", resp.StatusCode)
	}
}

func TestTransfers__getUserTransfers(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	repo := &sqliteTransferRepo{
		db:  db.db,
		log: log.NewNopLogger(),
	}

	amt, _ := NewAmount("USD", "12.42")
	userId := nextID()
	req := transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Customer:               CustomerID("customer"),
		CustomerDepository:     DepositoryID("customer"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
	}

	if _, err := repo.createUserTransfers(userId, []transferRequest{req}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/transfers", nil)
	r.Header.Set("x-user-id", userId)

	getUserTransfers(repo)(w, r)
	w.Flush()

	if w.Code != 200 {
		t.Errorf("got %d", w.Code)
	}

	var transfers []*Transfer
	if err := json.Unmarshal(w.Body.Bytes(), &transfers); err != nil {
		t.Error(err)
	}
	if len(transfers) != 1 {
		t.Fatalf("got %d transfers=%v", len(transfers), transfers)
	}
	if transfers[0].ID == "" {
		t.Errorf("transfers[0]=%v", transfers[0])
	}
	if v := transfers[0].Amount.String(); v != "USD 12.42" {
		t.Errorf("got %q", v)
	}
}

func TestTransfers__ABA(t *testing.T) {
	routingNumber := "231380104"
	if v := aba8(routingNumber); v != "23138010" {
		t.Errorf("got %s", v)
	}
	if v := abaCheckDigit(routingNumber); v != "4" {
		t.Errorf("got %s", v)
	}
}

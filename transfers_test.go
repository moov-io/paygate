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
	"time"

	"github.com/moov-io/base"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestTransfers__transferRequest(t *testing.T) {
	req := transferRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestTransfer__validate(t *testing.T) {
	amt, _ := NewAmount("USD", "27.12")
	transfer := &Transfer{
		ID:                     TransferID(nextID()),
		Type:                   PullTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Customer:               CustomerID("customer"),
		CustomerDepository:     DepositoryID("customer"),
		Description:            "test transfer",
		StandardEntryClassCode: "PPD",
		Status:                 TransferPending,
	}

	if err := transfer.validate(); err != nil {
		t.Errorf("transfer isn't valid: %v", err)
	}

	// fail due to Amount
	transfer.Amount = Amount{} // zero value
	if err := transfer.validate(); err == nil {
		t.Error("expected error, but got none")
	}
	transfer.Amount = *amt // reset state

	// fail due to Description
	transfer.Description = ""
	if err := transfer.validate(); err == nil {
		t.Error("expected error, but got none")
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
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Customer:               CustomerID("customer"),
		CustomerDepository:     DepositoryID("customer"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileId:                 "test-file",
	}

	if _, err := repo.createUserTransfers(userId, []*transferRequest{req}); err != nil {
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

	fileId, _ := repo.getFileIdForTransfer(transfers[0].ID, userId)
	if fileId != "test-file" {
		t.Error("no fileId found in transfers table")
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

func TestTransfers__writeResponse(t *testing.T) {
	w := httptest.NewRecorder()

	amt, _ := NewAmount("USD", "12.42")

	var transfers []*Transfer
	transfers = append(transfers, transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Customer:               CustomerID("customer"),
		CustomerDepository:     DepositoryID("customer"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileId:                 "test-file",
	}.asTransfer(nextID()))

	// Respond with one transfer, shouldn't be wrapped in an array
	writeResponse(w, 1, transfers)
	w.Flush()

	var singleResponse Transfer
	if err := json.NewDecoder(w.Body).Decode(&singleResponse); err != nil {
		t.Fatal(err)
	}
	if singleResponse.ID == "" {
		t.Errorf("empty transfer: %#v", singleResponse)
	}

	// Multiple requests, so wrap with an array
	w = httptest.NewRecorder()
	writeResponse(w, 2, transfers)
	w.Flush()

	var pluralResponse []Transfer
	if err := json.NewDecoder(w.Body).Decode(&pluralResponse); err != nil {
		t.Fatal(err)
	}
	if len(pluralResponse) != 1 {
		t.Errorf("got %d transfers", len(pluralResponse))
	}
	if pluralResponse[0].ID == "" {
		t.Errorf("empty transfer: %#v", pluralResponse[0])
	}
}

func TestTransfers__createTraceNumber(t *testing.T) {
	if v := createTraceNumber("121042882"); v == "" {
		t.Error("empty trace number")
	}
}

func TestTransfers_transferCursor(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	depRepo := &sqliteDepositoryRepo{db.db, log.NewNopLogger()}
	transferRepo := &sqliteTransferRepo{db.db, log.NewNopLogger()}

	userId := base.ID()
	amt := func(number string) Amount {
		amt, _ := NewAmount("USD", number)
		return *amt
	}

	dep := &Depository{
		ID:            DepositoryID(nextID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if err := depRepo.upsertUserDepository(userId, dep); err != nil {
		t.Fatal(err)
	}

	// Write transfers into database
	//
	// With the batch size low enough if more transfers are inserted within 1ms? than the batch size
	// the cursor will get stuck in an infinite loop. So we're inserting them at different times.
	//
	// TODO(adam): Will this become an issue?
	requests := []*transferRequest{
		{
			Type:                   PushTransfer,
			Amount:                 amt("12.12"),
			Originator:             OriginatorID("originator1"),
			OriginatorDepository:   DepositoryID("originator1"),
			Customer:               CustomerID("customer1"),
			CustomerDepository:     dep.ID, // CustomerDepository is read from a depositoryRepository
			Description:            "money1",
			StandardEntryClassCode: "PPD",
			fileId:                 "test-file1",
		},
	}
	if _, err := transferRepo.createUserTransfers(userId, requests); err != nil {
		t.Fatal(err)
	}
	requests = []*transferRequest{
		{
			Type:                   PullTransfer,
			Amount:                 amt("13.13"),
			Originator:             OriginatorID("originator2"),
			OriginatorDepository:   DepositoryID("originator2"),
			Customer:               CustomerID("customer2"),
			CustomerDepository:     dep.ID,
			Description:            "money2",
			StandardEntryClassCode: "PPD",
			fileId:                 "test-file2",
		},
	}
	if _, err := transferRepo.createUserTransfers(userId, requests); err != nil {
		t.Fatal(err)
	}
	requests = []*transferRequest{
		{
			Type:                   PushTransfer,
			Amount:                 amt("14.14"),
			Originator:             OriginatorID("originator3"),
			OriginatorDepository:   DepositoryID("originator3"),
			Customer:               CustomerID("customer3"),
			CustomerDepository:     dep.ID,
			Description:            "money3",
			StandardEntryClassCode: "PPD",
			fileId:                 "test-file3",
		},
	}
	if _, err := transferRepo.createUserTransfers(userId, requests); err != nil {
		t.Fatal(err)
	}

	// Now verify the cursor pulls those transfers out
	cur := transferRepo.getTransferCursor(2, depRepo) // batch size
	firstBatch, err := cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(firstBatch) != 2 {
		for i := range firstBatch {
			t.Errorf("firstBatch[%d]=%#v", i, firstBatch[i])
		}
	}
	secondBatch, err := cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(secondBatch) != 1 {
		for i := range secondBatch {
			t.Errorf("secondBatch[%d]=%#v", i, secondBatch[i])
		}
	}
}

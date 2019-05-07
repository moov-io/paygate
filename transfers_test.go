// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type testTransferRouter struct {
	*transferRouter

	ach       *achclient.ACH
	achServer *httptest.Server
	glClient  GLClient
}

func (r *testTransferRouter) close() {
	if r != nil && r.achServer != nil {
		r.achServer.Close()
	}
}

func createTestTransferRouter(
	dep depositoryRepository,
	evt eventRepository,
	rec receiverRepository,
	ori originatorRepository,
	xfr transferRepository,

	routes ...func(*mux.Router), // test ACH server routes
) *testTransferRouter {

	ach, _, achServer := achclient.MockClientServer("test", routes...)
	glClient := &testGLClient{}

	return &testTransferRouter{
		transferRouter: &transferRouter{
			logger:             log.NewNopLogger(),
			depRepo:            dep,
			eventRepo:          evt,
			receiverRepository: rec,
			origRepo:           ori,
			transferRepo:       xfr,
			achClientFactory: func(_ string) *achclient.ACH {
				return ach
			},
			glClient: glClient,
		},
		ach:       ach,
		achServer: achServer,
		glClient:  glClient,
	}
}

type mockTransferRepository struct {
	xfer   *Transfer
	fileId string

	err error
}

func (r *mockTransferRepository) getUserTransfers(userId string) ([]*Transfer, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.xfer != nil {
		return []*Transfer{r.xfer}, nil
	}
	return nil, nil
}

func (r *mockTransferRepository) getUserTransfer(id TransferID, userId string) (*Transfer, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.xfer, nil
}

func (r *mockTransferRepository) getFileIdForTransfer(id TransferID, userId string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.fileId, nil
}

func (r *mockTransferRepository) getTransferCursor(batchSize int, depRepo depositoryRepository) *transferCursor {
	return nil // TODO?
}

func (r *mockTransferRepository) markTransferAsMerged(id TransferID, filename string) error {
	return r.err
}

func (r *mockTransferRepository) createUserTransfers(userId string, requests []*transferRequest) ([]*Transfer, error) {
	if r.err != nil {
		return nil, r.err
	}
	var transfers []*Transfer
	for i := range requests {
		transfers = append(transfers, requests[i].asTransfer(base.ID()))
	}
	return transfers, nil
}

func (r *mockTransferRepository) deleteUserTransfer(id TransferID, userId string) error {
	return r.err
}

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
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
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
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
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
	if req.Receiver != "receiver" {
		t.Error(req.Receiver)
	}
	if req.ReceiverDepository != "receiver" {
		t.Error(req.ReceiverDepository)
	}
	if req.Description != "paycheck" {
		t.Error(req.Description)
	}
	if req.StandardEntryClassCode != "PPD" {
		t.Error(req.StandardEntryClassCode)
	}
}

func TestTransfers__idempotency(t *testing.T) {
	// The repositories aren't used, aka idempotency check needs to be first.
	xferRouter := createTestTransferRouter(nil, nil, nil, nil, nil)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.registerRoutes(router)

	req := httptest.NewRequest("POST", "/transfers", nil)
	req.Header.Set("x-idempotency-key", "key")
	req.Header.Set("x-user-id", "user")

	// mark the key as seen
	if seen := inmemIdempotentRecorder.SeenBefore("key"); seen {
		t.Errorf("shouldn't have been seen before")
	}

	// make our request
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__getUserTransfer(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	repo := &sqliteTransferRepo{db.db, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "18.61")
	userId := nextID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileId:                 "test-file",
	}

	xfers, err := repo.createUserTransfers(userId, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(xfers) != 1 {
		t.Errorf("got %d transfers", len(xfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/transfers/%s", xfers[0].ID), nil)
	r.Header.Set("x-user-id", userId)

	xferRouter := createTestTransferRouter(nil, nil, nil, nil, repo)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.registerRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)

	}

	var transfer Transfer
	if err := json.Unmarshal(w.Body.Bytes(), &transfer); err != nil {
		t.Error(err)
	}
	if transfer.ID == "" {
		t.Fatal("failed to parse Transfer")
	}
	if v := transfer.Amount.String(); v != "USD 18.61" {
		t.Errorf("got %q", v)
	}

	fileId, _ := repo.getFileIdForTransfer(transfer.ID, userId)
	if fileId != "test-file" {
		t.Error("no fileId found in transfers table")
	}

	// have our repository error and verify we get non-200's
	xferRouter.transferRepo = &mockTransferRepository{err: errors.New("bad error")}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__getUserTransfers(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	repo := &sqliteTransferRepo{db.db, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "12.42")
	userId := nextID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
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

	xferRouter := createTestTransferRouter(nil, nil, nil, nil, repo)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.registerRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
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

	// have our repository error and verify we get non-200's
	xferRouter.transferRepo = &mockTransferRepository{err: errors.New("bad error")}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__deleteUserTransfer(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	repo := &sqliteTransferRepo{db.db, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "12.42")
	userId := nextID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileId:                 "test-file",
	}

	transfers, err := repo.createUserTransfers(userId, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Errorf("got %d transfers", len(transfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", fmt.Sprintf("/transfers/%s", transfers[0].ID), nil)
	r.Header.Set("x-user-id", userId)

	xferRouter := createTestTransferRouter(nil, nil, nil, nil, repo, achclient.AddDeleteRoute)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.registerRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}

	// have our repository error and verify we get non-200's
	xferRouter.transferRepo = &mockTransferRepository{err: errors.New("bad error")}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__validateUserTransfer(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	repo := &sqliteTransferRepo{db.db, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "32.41")
	userId := nextID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileId:                 "test-file",
	}
	transfers, err := repo.createUserTransfers(userId, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Errorf("got %d transfers", len(transfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", fmt.Sprintf("/transfers/%s/failed", transfers[0].ID), nil)
	r.Header.Set("x-user-id", userId)

	xferRouter := createTestTransferRouter(nil, nil, nil, nil, repo, achclient.AddValidateRoute)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.registerRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}

	// have our repository error and verify we get non-200's
	mockRepo := &mockTransferRepository{err: errors.New("bad error")}
	xferRouter.transferRepo = mockRepo

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d", w.Code)
	}

	// no repository error, but pretend the ACH file is invalid
	mockRepo.err = nil
	xferRouter2 := createTestTransferRouter(nil, nil, nil, nil, repo, achclient.AddInvalidRoute)

	router = mux.NewRouter()
	xferRouter2.registerRoutes(router)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__getUserTransferFiles(t *testing.T) {
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()

	repo := &sqliteTransferRepo{db.db, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "32.41")
	userId := nextID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileId:                 "test-file",
	}
	transfers, err := repo.createUserTransfers(userId, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Errorf("got %d transfers", len(transfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", fmt.Sprintf("/transfers/%s/files", transfers[0].ID), nil)
	r.Header.Set("x-user-id", userId)

	xferRouter := createTestTransferRouter(nil, nil, nil, nil, repo, achclient.AddGetFileRoute)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.registerRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}

	bs, _ := ioutil.ReadAll(w.Body)
	bs = bytes.TrimSpace(bs)

	// Verify it's an array returned
	if !bytes.HasPrefix(bs, []byte("[")) || !bytes.HasSuffix(bs, []byte("]")) {
		t.Fatalf("unknown response: %v", string(bs))
	}

	// ach.FileFromJSON doesn't handle multiple files, so for now just break up the array
	file, err := ach.FileFromJSON(bs[1 : len(bs)-1]) // crude strip of [ and ]
	if err != nil || file == nil {
		t.Errorf("file=%v err=%v", file, err)
	}

	// have our repository error and verify we get non-200's
	mockRepo := &mockTransferRepository{err: errors.New("bad error")}
	xferRouter.transferRepo = mockRepo

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d", w.Code)
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

	// 10 digit from ACH server
	if v := aba8("0123456789"); v != "12345678" {
		t.Errorf("got %s", v)
	}
	if v := abaCheckDigit("0123456789"); v != "9" {
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
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileId:                 "test-file",
	}.asTransfer(nextID()))

	// Respond with one transfer, shouldn't be wrapped in an array
	writeResponse(log.NewNopLogger(), w, 1, transfers)
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
	writeResponse(log.NewNopLogger(), w, 2, transfers)
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
			Receiver:               ReceiverID("receiver1"),
			ReceiverDepository:     dep.ID, // ReceiverDepository is read from a depositoryRepository
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
			Receiver:               ReceiverID("receiver2"),
			ReceiverDepository:     dep.ID,
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
			Receiver:               ReceiverID("receiver3"),
			ReceiverDepository:     dep.ID,
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

func TestTransfers_markTransferAsMerged(t *testing.T) {
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
			Receiver:               ReceiverID("receiver1"),
			ReceiverDepository:     dep.ID, // ReceiverDepository is read from a depositoryRepository
			Description:            "money1",
			StandardEntryClassCode: "PPD",
			fileId:                 "test-file1",
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
	if len(firstBatch) != 1 {
		for i := range firstBatch {
			t.Errorf("firstBatch[%d]=%#v", i, firstBatch[i])
		}
	}

	// mark our transfer as merged, so we don't see it (in a new transferCursor we create)
	if err := transferRepo.markTransferAsMerged(firstBatch[0].ID, "merged-file.ach"); err != nil {
		t.Fatal(err)
	}

	// re-create our transferCursor and see the transfer ignored
	// plus add a second transfer and ensure we get that
	requests = []*transferRequest{
		{
			Type:                   PullTransfer,
			Amount:                 amt("13.13"),
			Originator:             OriginatorID("originator2"),
			OriginatorDepository:   DepositoryID("originator2"),
			Receiver:               ReceiverID("receiver2"),
			ReceiverDepository:     dep.ID,
			Description:            "money2",
			StandardEntryClassCode: "PPD",
			fileId:                 "test-file2",
		},
	}
	if _, err := transferRepo.createUserTransfers(userId, requests); err != nil {
		t.Fatal(err)
	}
	cur = transferRepo.getTransferCursor(2, depRepo) // batch size
	firstBatch, err = cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(firstBatch) != 1 {
		for i := range firstBatch {
			t.Errorf("firstBatch[%d].ID=%v amount=%v", i, firstBatch[i].ID, firstBatch[i].Amount.String())
		}
	}
	if firstBatch[0].Amount.String() != "USD 13.13" {
		t.Errorf("got %v", firstBatch[0].Amount.String())
	}
}

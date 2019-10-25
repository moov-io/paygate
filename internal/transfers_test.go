// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accounts "github.com/moov-io/accounts/client"
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type testTransferRouter struct {
	*TransferRouter

	ach            *achclient.ACH
	achServer      *httptest.Server
	accountsClient AccountsClient
}

func (r *testTransferRouter) close() {
	if r != nil && r.achServer != nil {
		r.achServer.Close()
	}
}

func CreateTestTransferRouter(
	dep DepositoryRepository,
	evt EventRepository,
	rec receiverRepository,
	ori originatorRepository,
	xfr TransferRepository,

	routes ...func(*mux.Router), // test ACH server routes
) *testTransferRouter {

	ach, _, achServer := achclient.MockClientServer("test", routes...)
	accountsClient := &testAccountsClient{}

	return &testTransferRouter{
		TransferRouter: &TransferRouter{
			logger:             log.NewNopLogger(),
			depRepo:            dep,
			eventRepo:          evt,
			receiverRepository: rec,
			origRepo:           ori,
			transferRepo:       xfr,
			achClientFactory: func(_ string) *achclient.ACH {
				return ach
			},
			accountsClient: accountsClient,
		},
		ach:            ach,
		achServer:      achServer,
		accountsClient: accountsClient,
	}
}

func TestTransfers__transferRequest(t *testing.T) {
	req := transferRequest{}
	if err := req.missingFields(); err == nil {
		t.Error("expected error")
	}
}

func TestTransfer__json(t *testing.T) {
	xfer := Transfer{
		ID:            TransferID("xfer"),
		Receiver:      ReceiverID("receiver"),
		TransactionID: "transacion",
		UserID:        "user",
		ReturnCode: &ach.ReturnCode{
			Code:   "R02",
			Reason: "Account Closed",
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&xfer); err != nil {
		t.Fatal(err)
	}

	v := buf.String()

	if !strings.Contains(v, `{"id":"xfer",`) {
		t.Error(v)
	}
	if !strings.Contains(v, `"receiver":"receiver",`) {
		t.Error(v)
	}
	if strings.Contains(v, `transaction`) {
		t.Error(v)
	}
	if strings.Contains(v, `user`) {
		t.Error(v)
	}
	if !strings.Contains(v, "R02") {
		t.Error(v)
	}
}

func TestTransfer__validate(t *testing.T) {
	amt, _ := NewAmount("USD", "27.12")
	transfer := &Transfer{
		ID:                     TransferID(base.ID()),
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
	in := []byte(fmt.Sprintf(`"%v"`, base.ID()))
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
	in := []byte(fmt.Sprintf(`"%v"`, base.ID()))
	if err := json.Unmarshal(in, &ts); err == nil {
		t.Error("expected error")
	}
}

func TestTransfers__asTransferJSON(t *testing.T) {
	body := strings.NewReader(`{
  "transferType": "push",
  "amount": "USD 99.99",
  "originator": "32c95f289e18fb31a9a355c24ffa4ffc00a481e6",
  "originatorDepository": "ccac06454d87b6621bc62e07708ba9c342cd87ef",
  "receiver": "47c2c9e090a3417d9951eb8f0469a0d3fe7b3610",
  "receiverDepository": "8b6aadaddb25b961afd8cebbce7af306104a667c",
  "description": "Loan Pay",
  "standardEntryClassCode": "WEB",
  "sameDay": false,
  "WEBDetail": {
    "paymentInformation": "test payment",
    "paymentType": "single"
  }
}`)
	var req transferRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		t.Fatal(err)
	}
	xfer := req.asTransfer(base.ID())

	// marshal the Transfer back to JSON and verify only WEBDetail was written
	bs, err := json.MarshalIndent(xfer, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bs, []byte("CCDDetail")) {
		t.Errorf("Transfer contains CCDDetail: %v", string(bs))
	}
	if bytes.Contains(bs, []byte("IATDetail")) {
		t.Errorf("Transfer contains IATDetail: %v", string(bs))
	}
	if bytes.Contains(bs, []byte("TELDetail")) {
		t.Errorf("Transfer contains TELDetail: %v", string(bs))
	}
	if !bytes.Contains(bs, []byte("WEBDetail")) {
		t.Errorf("Transfer contains WEBDetail: %v", string(bs))
	}
}

func TestTransfers__asTransfer(t *testing.T) {
	body := strings.NewReader(`{
  "transferType": "push",
  "amount": "USD 99.99",
  "originator": "32c95f289e18fb31a9a355c24ffa4ffc00a481e6",
  "originatorDepository": "ccac06454d87b6621bc62e07708ba9c342cd87ef",
  "receiver": "47c2c9e090a3417d9951eb8f0469a0d3fe7b3610",
  "receiverDepository": "8b6aadaddb25b961afd8cebbce7af306104a667c",
  "description": "Loan Pay",
  "standardEntryClassCode": "WEB",
  "sameDay": false,
  "WEBDetail": {
    "paymentInformation": "test payment",
    "paymentType": "single"
  }
}`)
	var req transferRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		t.Fatal(err)
	}
	xfer := req.asTransfer(base.ID())
	if xfer.StandardEntryClassCode != "WEB" {
		t.Errorf("xfer.StandardEntryClassCode=%s", xfer.StandardEntryClassCode)
	}
	if xfer.CCDDetail != nil && xfer.CCDDetail.PaymentInformation != "" {
		t.Errorf("xfer.CCDDetail.PaymentInformation=%s", xfer.CCDDetail.PaymentInformation)
	}
	if xfer.IATDetail != nil && xfer.IATDetail.OriginatorName != "" {
		t.Errorf("xfer.IATDetail.OriginatorName=%s", xfer.IATDetail.OriginatorName)
	}
	if xfer.TELDetail != nil && xfer.TELDetail.PhoneNumber != "" {
		t.Errorf("xfer.TELDetail.PhoneNumber=%s", xfer.TELDetail.PhoneNumber)
	}
	if xfer.WEBDetail == nil || xfer.WEBDetail.PaymentInformation != "test payment" || xfer.WEBDetail.PaymentType != WEBSingle {
		t.Errorf("xfer.WEBDetail.PaymentInformation=%s xfer.WEBDetail.PaymentType=%s", xfer.WEBDetail.PaymentInformation, xfer.WEBDetail.PaymentType)
	}
}

// TestTransferRequest__asTransfer is a test to ensure we copy YYYDetail sub-objects properly in (transferRequest).asTransfer(..)
func TestTransferRequest__asTransfer(t *testing.T) {
	// CCD
	req := transferRequest{
		StandardEntryClassCode: "CCD",
		CCDDetail: &CCDDetail{
			PaymentInformation: "foo",
		},
	}
	xfer := req.asTransfer(base.ID())
	if xfer.CCDDetail == nil || xfer.CCDDetail.PaymentInformation != "foo" {
		t.Errorf("xfer.CCDDetail=%#v", xfer.CCDDetail)
	}

	// IAT
	req = transferRequest{
		StandardEntryClassCode: "IAT",
		IATDetail: &IATDetail{
			ODFIName: "moov bank",
		},
	}
	xfer = req.asTransfer(base.ID())
	if xfer.CCDDetail != nil { // check previous case
		t.Fatal("xfer.CCDDetail=%#V", xfer.CCDDetail)
	}
	if xfer.IATDetail == nil || xfer.IATDetail.ODFIName != "moov bank" {
		t.Errorf("xfer.IATDetail=%#v", xfer.IATDetail)
	}

	// TEL
	req = transferRequest{
		StandardEntryClassCode: "TEL",
		TELDetail: &TELDetail{
			PhoneNumber: "1",
			PaymentType: TELSingle,
		},
	}
	xfer = req.asTransfer(base.ID())
	if xfer.IATDetail != nil { // check previous case
		t.Fatal("xfer.IATDetail=%#V", xfer.IATDetail)
	}
	if xfer.TELDetail == nil || xfer.TELDetail.PhoneNumber != "1" || xfer.TELDetail.PaymentType != TELSingle {
		t.Errorf("xfer.TELDetail=%#v", xfer.TELDetail)
	}

	// WEB
	req = transferRequest{
		StandardEntryClassCode: "WEB",
		WEBDetail: &WEBDetail{
			PaymentInformation: "bar",
			PaymentType:        WEBSingle,
		},
	}
	xfer = req.asTransfer(base.ID())
	if xfer.TELDetail != nil { // check previous case
		t.Fatal("xfer.TELDetail=%#V", xfer.TELDetail)
	}
	if xfer.WEBDetail == nil || xfer.WEBDetail.PaymentInformation != "bar" || xfer.WEBDetail.PaymentType != WEBSingle {
		t.Errorf("xfer.WEBDetail=%#v", xfer.WEBDetail)
	}
}

func TestTransfers__read(t *testing.T) {
	amt, _ := NewAmount("USD", "27.12")
	request := transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "paycheck",
		StandardEntryClassCode: "PPD",
	}
	check := func(t *testing.T, req *transferRequest) {
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

	// Read a single transferRequest object
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(request); err != nil {
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
	check(t, requests[0])

	// Read an array of transferRequest objects
	if err := json.NewEncoder(&buf).Encode([]transferRequest{request}); err != nil {
		t.Fatal(err)
	}
	requests, err = readTransferRequests(&http.Request{
		Body: ioutil.NopCloser(&buf),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Error(requests)
	}
	check(t, requests[0])
}

func TestTransfers__create(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	logger := log.NewNopLogger()
	now := base.NewTime(time.Now())

	depRepo := &MockDepositoryRepository{
		Depositories: []*Depository{
			{
				ID:            DepositoryID("originator"),
				BankName:      "orig bank",
				Holder:        "orig",
				HolderType:    Individual,
				Type:          Checking,
				RoutingNumber: "121421212",
				AccountNumber: "1321",
				Status:        DepositoryVerified,
				Metadata:      "metadata",
				Created:       now,
				Updated:       now,
			},
			{
				ID:            DepositoryID("receiver"),
				BankName:      "receiver bank",
				Holder:        "receiver",
				HolderType:    Individual,
				Type:          Checking,
				RoutingNumber: "121421212",
				AccountNumber: "323431",
				Status:        DepositoryVerified,
				Metadata:      "metadata",
				Created:       now,
				Updated:       now,
			},
		},
	}
	eventRepo := NewEventRepo(logger, db.DB)
	recRepo := &mockReceiverRepository{
		receivers: []*Receiver{
			{
				ID:                ReceiverID("receiver"),
				Email:             "foo@moov.io",
				DefaultDepository: DepositoryID("receiver"),
				Status:            ReceiverVerified,
				Metadata:          "other",
				Created:           now,
				Updated:           now,
			},
		},
	}
	origRepo := &mockOriginatorRepository{
		originators: []*Originator{
			{
				ID:                OriginatorID("originator"),
				DefaultDepository: DepositoryID("originator"),
				Identification:    "id",
				Metadata:          "other",
				Created:           now,
				Updated:           now,
			},
		},
	}
	repo := &SQLTransferRepo{db.DB, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "18.61")
	request := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()

	router := CreateTestTransferRouter(depRepo, eventRepo, recRepo, origRepo, repo, func(r *mux.Router) {
		achclient.AddCreateRoute(w, r)
	})
	defer router.close()
	router.accountsClient = nil
	router.TransferRouter.accountsClient = nil

	req, _ := http.NewRequest("POST", "/transfers", &body)
	req.Header.Set("x-user-id", "test")
	router.createUserTransfers()(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP statu codes: %d: %s", w.Code, w.Body.String())
	}
}

func TestTransfers__idempotency(t *testing.T) {
	// The repositories aren't used, aka idempotency check needs to be first.
	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, nil)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)

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
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLTransferRepo{db.DB, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "18.61")
	userID := base.ID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}

	xfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(xfers) != 1 {
		t.Errorf("got %d transfers", len(xfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/transfers/%s", xfers[0].ID), nil)
	r.Header.Set("x-user-id", userID)

	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, repo)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
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

	fileID, _ := repo.GetFileIDForTransfer(transfer.ID, userID)
	if fileID != "test-file" {
		t.Error("no fileID found in transfers table")
	}

	// have our repository error and verify we get non-200's
	xferRouter.transferRepo = &MockTransferRepository{Err: errors.New("bad error")}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__getUserTransfers(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLTransferRepo{db.DB, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "12.42")
	userID := base.ID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}

	if _, err := repo.createUserTransfers(userID, []*transferRequest{req}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/transfers", nil)
	r.Header.Set("x-user-id", userID)

	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, repo)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
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

	fileID, _ := repo.GetFileIDForTransfer(transfers[0].ID, userID)
	if fileID != "test-file" {
		t.Error("no fileID found in transfers table")
	}

	// have our repository error and verify we get non-200's
	xferRouter.transferRepo = &MockTransferRepository{Err: errors.New("bad error")}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__deleteUserTransfer(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLTransferRepo{db.DB, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "12.42")
	userID := base.ID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}

	transfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Errorf("got %d transfers", len(transfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", fmt.Sprintf("/transfers/%s", transfers[0].ID), nil)
	r.Header.Set("x-user-id", userID)

	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, repo, achclient.AddDeleteRoute)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d: %s", w.Code, w.Body.String())
	}

	// have our repository error and verify we get non-200's
	xferRouter.transferRepo = &MockTransferRepository{Err: errors.New("bad error")}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__validateUserTransfer(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLTransferRepo{db.DB, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "32.41")
	userID := base.ID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}
	transfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Errorf("got %d transfers", len(transfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", fmt.Sprintf("/transfers/%s/failed", transfers[0].ID), nil)
	r.Header.Set("x-user-id", userID)

	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, repo, achclient.AddValidateRoute)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}

	// have our repository error and verify we get non-200's
	mockRepo := &MockTransferRepository{Err: errors.New("bad error")}
	xferRouter.transferRepo = mockRepo

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}

	// no repository error, but pretend the ACH file is invalid
	mockRepo.Err = nil
	xferRouter2 := CreateTestTransferRouter(nil, nil, nil, nil, repo, achclient.AddInvalidRoute)

	router = mux.NewRouter()
	xferRouter2.RegisterRoutes(router)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__getUserTransferFiles(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLTransferRepo{db.DB, log.NewNopLogger()}

	amt, _ := NewAmount("USD", "32.41")
	userID := base.ID()
	req := &transferRequest{
		Type:                   PushTransfer,
		Amount:                 *amt,
		Originator:             OriginatorID("originator"),
		OriginatorDepository:   DepositoryID("originator"),
		Receiver:               ReceiverID("receiver"),
		ReceiverDepository:     DepositoryID("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}
	transfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Errorf("got %d transfers", len(transfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", fmt.Sprintf("/transfers/%s/files", transfers[0].ID), nil)
	r.Header.Set("x-user-id", userID)

	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, repo, achclient.AddGetFileRoute)
	defer xferRouter.close()

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
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
	mockRepo := &MockTransferRepository{Err: errors.New("bad error")}
	xferRouter.transferRepo = mockRepo

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
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
		fileID:                 "test-file",
	}.asTransfer(base.ID()))

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
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	depRepo := &SQLDepositoryRepo{db.DB, log.NewNopLogger()}
	transferRepo := &SQLTransferRepo{db.DB, log.NewNopLogger()}

	userID := base.ID()
	amt := func(number string) Amount {
		amt, _ := NewAmount("USD", number)
		return *amt
	}

	dep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
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
			OriginatorDepository:   dep.ID, // OriginatorDepository is read from a depositoryRepository
			Receiver:               ReceiverID("receiver1"),
			ReceiverDepository:     DepositoryID("receiver1"),
			Description:            "money1",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file1",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}
	requests = []*transferRequest{
		{
			Type:                   PullTransfer,
			Amount:                 amt("13.13"),
			Originator:             OriginatorID("originator2"),
			OriginatorDepository:   dep.ID,
			Receiver:               ReceiverID("receiver2"),
			ReceiverDepository:     DepositoryID("receiver2"),
			Description:            "money2",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file2",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}
	requests = []*transferRequest{
		{
			Type:                   PushTransfer,
			Amount:                 amt("14.14"),
			Originator:             OriginatorID("originator3"),
			OriginatorDepository:   dep.ID,
			Receiver:               ReceiverID("receiver3"),
			ReceiverDepository:     DepositoryID("receiver3"),
			Description:            "money3",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file3",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}

	// Now verify the cursor pulls those transfers out
	cur := transferRepo.GetTransferCursor(2, depRepo) // batch size
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

func TestTransfers_MarkTransferAsMerged(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	depRepo := &SQLDepositoryRepo{db.DB, log.NewNopLogger()}
	transferRepo := &SQLTransferRepo{db.DB, log.NewNopLogger()}

	userID := base.ID()
	amt := func(number string) Amount {
		amt, _ := NewAmount("USD", number)
		return *amt
	}

	dep := &Depository{
		ID:            DepositoryID(base.ID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "123",
		AccountNumber: "151",
		Status:        DepositoryUnverified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
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
			OriginatorDepository:   dep.ID, // ReceiverDepository is read from a depositoryRepository
			Receiver:               ReceiverID("receiver1"),
			ReceiverDepository:     DepositoryID("receiver1"),
			Description:            "money1",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file1",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}

	// Now verify the cursor pulls those transfers out
	cur := transferRepo.GetTransferCursor(2, depRepo) // batch size
	firstBatch, err := cur.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(firstBatch) != 1 {
		for i := range firstBatch {
			t.Errorf("firstBatch[%d]=%#v", i, firstBatch[i])
		}
		t.Fatalf("firstBatch: %#v", firstBatch)
	}

	// mark our transfer as merged, so we don't see it (in a new transferCursor we create)
	if err := transferRepo.MarkTransferAsMerged(firstBatch[0].ID, "merged-file.ach", "traceNumber"); err != nil {
		t.Fatal(err)
	}

	// re-create our transferCursor and see the transfer ignored
	// plus add a second transfer and ensure we get that
	requests = []*transferRequest{
		{
			Type:                   PullTransfer,
			Amount:                 amt("13.13"),
			Originator:             OriginatorID("originator2"),
			OriginatorDepository:   dep.ID,
			Receiver:               ReceiverID("receiver2"),
			ReceiverDepository:     DepositoryID("receiver2"),
			Description:            "money2",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file2",
		},
	}
	if _, err := transferRepo.createUserTransfers(userID, requests); err != nil {
		t.Fatal(err)
	}
	cur = transferRepo.GetTransferCursor(2, depRepo) // batch size
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

func TestTransfers__createTransactionLines(t *testing.T) {
	orig := &accounts.Account{ID: base.ID()}
	rec := &accounts.Account{ID: base.ID()}
	amt, _ := NewAmount("USD", "12.53")

	lines := createTransactionLines(orig, rec, *amt, PushTransfer)
	if len(lines) != 2 {
		t.Errorf("got %d lines: %v", len(lines), lines)
	}

	// First transactionLine
	if lines[0].AccountID != orig.ID {
		t.Errorf("lines[0].AccountID=%s", lines[0].AccountID)
	}
	if !strings.EqualFold(lines[0].Purpose, "ACHDebit") {
		t.Errorf("lines[0].Purpose=%s", lines[0].Purpose)
	}
	if lines[0].Amount != 1253 {
		t.Errorf("lines[0].Amount=%d", lines[0].Amount)
	}

	// Second transactionLine
	if lines[1].AccountID != rec.ID {
		t.Errorf("lines[1].AccountID=%s", lines[1].AccountID)
	}
	if !strings.EqualFold(lines[1].Purpose, "ACHCredit") {
		t.Errorf("lines[1].Purpose=%s", lines[1].Purpose)
	}
	if lines[1].Amount != 1253 {
		t.Errorf("lines[1].Amount=%d", lines[1].Amount)
	}

	// flip the TransferType
	lines = createTransactionLines(orig, rec, *amt, PullTransfer)
	if !strings.EqualFold(lines[0].Purpose, "ACHCredit") {
		t.Errorf("lines[0].Purpose=%s", lines[0].Purpose)
	}
	if !strings.EqualFold(lines[1].Purpose, "ACHDebit") {
		t.Errorf("lines[1].Purpose=%s", lines[1].Purpose)
	}
}

func TestTransfers__postAccountTransaction(t *testing.T) {
	transferRepo := &MockTransferRepository{}

	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, transferRepo)
	defer xferRouter.close()

	if a, ok := xferRouter.accountsClient.(*testAccountsClient); ok {
		a.accounts = []accounts.Account{
			{
				ID: base.ID(), // Just a stub, the fields aren't checked in this test
			},
		}
		a.transaction = &accounts.Transaction{ID: base.ID()}
	} else {
		t.Fatalf("unknown AccountsClient: %T", xferRouter.accountsClient)
	}

	amt, _ := NewAmount("USD", "63.21")
	origDep := &Depository{
		AccountNumber: "214124124",
		RoutingNumber: "1215125151",
		Type:          Checking,
	}
	recDep := &Depository{
		AccountNumber: "212142",
		RoutingNumber: "1215125151",
		Type:          Savings,
	}

	userID, requestID := base.ID(), base.ID()
	tx, err := xferRouter.postAccountTransaction(userID, origDep, recDep, *amt, PullTransfer, requestID)
	if err != nil {
		t.Fatal(err)
	}
	if tx == nil {
		t.Errorf("nil accounts.Transaction")
	}
}

func TestTransfers__UpdateTransferStatus(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo TransferRepository) {
		amt, _ := NewAmount("USD", "32.92")
		userID := base.ID()
		req := &transferRequest{
			Type:                   PushTransfer,
			Amount:                 *amt,
			Originator:             OriginatorID("originator"),
			OriginatorDepository:   DepositoryID("originator"),
			Receiver:               ReceiverID("receiver"),
			ReceiverDepository:     DepositoryID("receiver"),
			Description:            "money",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file",
		}
		transfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.UpdateTransferStatus(transfers[0].ID, TransferReclaimed); err != nil {
			t.Fatal(err)
		}

		xfer, err := repo.getUserTransfer(transfers[0].ID, userID)
		if err != nil {
			t.Error(err)
		}
		if xfer.Status != TransferReclaimed {
			t.Errorf("got status %s", xfer.Status)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLTransferRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLTransferRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestTransfers__transactionID(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, db *sql.DB) {
		userID := base.ID()
		transactionID := base.ID() // field we care about
		amt, _ := NewAmount("USD", "51.21")

		repo := &SQLTransferRepo{db, log.NewNopLogger()}
		requests := []*transferRequest{
			{
				Type:                   PullTransfer,
				Amount:                 *amt,
				Originator:             OriginatorID("originator"),
				OriginatorDepository:   DepositoryID("originatorDep"),
				Receiver:               ReceiverID("receiver"),
				ReceiverDepository:     DepositoryID("receiverDep"),
				Description:            "money2",
				StandardEntryClassCode: "PPD",
				transactionID:          transactionID,
			},
		}
		if _, err := repo.createUserTransfers(userID, requests); err != nil {
			t.Fatal(err)
		}

		transfers, err := repo.getUserTransfers(userID)
		if err != nil || len(transfers) != 1 {
			t.Errorf("got %d Transfers (error=%v): %v", len(transfers), err, transfers)
		}

		query := `select transaction_id from transfers where transfer_id = ?`
		stmt, err := db.Prepare(query)
		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()

		var txID string
		row := stmt.QueryRow(transfers[0].ID)
		if err := row.Scan(&txID); err != nil {
			t.Fatal(err)
		}
		if txID != transactionID {
			t.Errorf("incorrect transactionID: %s vs %s", txID, transactionID)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, sqliteDB.DB)

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, mysqlDB.DB)
}

func TestTransfers__LookupTransferFromReturn(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo TransferRepository) {
		amt, _ := NewAmount("USD", "32.92")
		userID := base.ID()
		req := &transferRequest{
			Type:                   PushTransfer,
			Amount:                 *amt,
			Originator:             OriginatorID("originator"),
			OriginatorDepository:   DepositoryID("originator"),
			Receiver:               ReceiverID("receiver"),
			ReceiverDepository:     DepositoryID("receiver"),
			Description:            "money",
			StandardEntryClassCode: "PPD",
			fileID:                 "test-file",
		}
		transfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
		if err != nil {
			t.Fatal(err)
		}

		// set metadata after transfer is merged into an ACH file for the FED
		if err := repo.MarkTransferAsMerged(transfers[0].ID, "merged.ach", "traceNumber"); err != nil {
			t.Fatal(err)
		}

		// Now grab the transfer back
		xfer, err := repo.LookupTransferFromReturn("PPD", amt, "traceNumber", time.Now()) // EffectiveEntryDate is bounded by start and end of a day
		if err != nil {
			t.Fatal(err)
		}
		if xfer.ID != transfers[0].ID || xfer.UserID != userID {
			t.Errorf("found other transfer=%q user=(%q vs %q)", xfer.ID, xfer.UserID, userID)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLTransferRepo{sqliteDB.DB, log.NewNopLogger()})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLTransferRepo{mysqlDB.DB, log.NewNopLogger()})
}

func TestTransfers__SetReturnCode(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, db *sql.DB) {
		userID := base.ID()
		returnCode := "R17"
		amt, _ := NewAmount("USD", "51.21")

		repo := &SQLTransferRepo{db, log.NewNopLogger()}
		requests := []*transferRequest{
			{
				Type:                   PullTransfer,
				Amount:                 *amt,
				Originator:             OriginatorID("originator"),
				OriginatorDepository:   DepositoryID("originatorDep"),
				Receiver:               ReceiverID("receiver"),
				ReceiverDepository:     DepositoryID("receiverDep"),
				Description:            "money2",
				StandardEntryClassCode: "PPD",
			},
		}
		if _, err := repo.createUserTransfers(userID, requests); err != nil {
			t.Fatal(err)
		}

		transfers, err := repo.getUserTransfers(userID)
		if err != nil || len(transfers) != 1 {
			t.Errorf("got %d Transfers (error=%v): %v", len(transfers), err, transfers)
		}

		// Set ReturnCode
		if err := repo.SetReturnCode(transfers[0].ID, returnCode); err != nil {
			t.Fatal(err)
		}

		// Verify
		transfers, err = repo.getUserTransfers(userID)
		if err != nil || len(transfers) != 1 {
			t.Errorf("got %d Transfers (error=%v): %v", len(transfers), err, transfers)
		}
		if transfers[0].ReturnCode == nil {
			t.Fatal("expected ReturnCode")
		}
		if transfers[0].ReturnCode.Code != returnCode {
			t.Errorf("transfers[0].ReturnCode.Code=%s", transfers[0].ReturnCode.Code)
		}

		t.Logf("%#v", transfers[0].ReturnCode)
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, sqliteDB.DB)

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, mysqlDB.DB)
}

func TestTransfers__constructACHFile(t *testing.T) {
	// The fields on each struct are minimized to help throttle this file's size
	receiverDep := &Depository{
		BankName:      "foo bank",
		RoutingNumber: "121042882",
	}
	receiver := &Receiver{Status: ReceiverVerified}
	origDep := &Depository{
		BankName:      "foo bank",
		RoutingNumber: "231380104",
	}
	orig := &Originator{}
	transfer := &Transfer{
		Type:                   PushTransfer,
		Status:                 TransferPending,
		StandardEntryClassCode: "AAA", // invalid
	}

	file, err := constructACHFile("", "", "", transfer, receiver, receiverDep, orig, origDep)
	if err == nil || file != nil {
		t.Fatalf("expected error, got file=%#v", file)
	}
	if !strings.Contains(err.Error(), "unsupported SEC code: AAA") {
		t.Errorf("unexpected error: %v", err)
	}
}

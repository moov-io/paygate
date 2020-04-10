// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/accounts"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/gateways"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/originators"
	"github.com/moov-io/paygate/internal/receivers"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/internal/util"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type testTransferRouter struct {
	*TransferRouter

	accountsClient accounts.Client
}

func CreateTestTransferRouter(
	depRepo depository.Repository,
	eventRepo events.Repository,
	gatewayRepo gateways.Repository,
	recRepo receivers.Repository,
	origRepo originators.Repository,
	xfr Repository,
) *testTransferRouter {

	limits, _ := ParseLimits(OneDayLimit(), SevenDayLimit(), ThirtyDayLimit())

	var db *sql.DB
	if rr, ok := xfr.(*SQLRepo); ok {
		db = rr.db
	}
	limiter := NewLimitChecker(log.NewNopLogger(), db, limits)

	accountsClient := &accounts.MockClient{}

	return &testTransferRouter{
		TransferRouter: &TransferRouter{
			logger:               log.NewNopLogger(),
			depRepo:              depRepo,
			eventRepo:            eventRepo,
			gatewayRepo:          gatewayRepo,
			receiverRepository:   recRepo,
			origRepo:             origRepo,
			transferRepo:         xfr,
			transferLimitChecker: limiter,
			accountsClient:       accountsClient,
		},
		accountsClient: accountsClient,
	}
}

func TestTransfers__transferRequest(t *testing.T) {
	req := transferRequest{}
	if err := req.missingFields(); err == nil {
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
	if xfer.WEBDetail == nil || xfer.WEBDetail.PaymentInformation != "test payment" || xfer.WEBDetail.PaymentType != model.WEBSingle {
		t.Errorf("xfer.WEBDetail.PaymentInformation=%s xfer.WEBDetail.PaymentType=%s", xfer.WEBDetail.PaymentInformation, xfer.WEBDetail.PaymentType)
	}
}

// TestTransferRequest__asTransfer is a test to ensure we copy YYYDetail sub-objects properly in (transferRequest).asTransfer(..)
func TestTransferRequest__asTransfer(t *testing.T) {
	// CCD
	req := transferRequest{
		StandardEntryClassCode: "CCD",
		CCDDetail: &model.CCDDetail{
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
		IATDetail: &model.IATDetail{
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
		TELDetail: &model.TELDetail{
			PhoneNumber: "1",
			PaymentType: model.TELSingle,
		},
	}
	xfer = req.asTransfer(base.ID())
	if xfer.IATDetail != nil { // check previous case
		t.Fatal("xfer.IATDetail=%#V", xfer.IATDetail)
	}
	if xfer.TELDetail == nil || xfer.TELDetail.PhoneNumber != "1" || xfer.TELDetail.PaymentType != model.TELSingle {
		t.Errorf("xfer.TELDetail=%#v", xfer.TELDetail)
	}

	// WEB
	req = transferRequest{
		StandardEntryClassCode: "WEB",
		WEBDetail: &model.WEBDetail{
			PaymentInformation: "bar",
			PaymentType:        model.WEBSingle,
		},
	}
	xfer = req.asTransfer(base.ID())
	if xfer.TELDetail != nil { // check previous case
		t.Fatal("xfer.TELDetail=%#V", xfer.TELDetail)
	}
	if xfer.WEBDetail == nil || xfer.WEBDetail.PaymentInformation != "bar" || xfer.WEBDetail.PaymentType != model.WEBSingle {
		t.Errorf("xfer.WEBDetail=%#v", xfer.WEBDetail)
	}
}

func TestTransfers__rejectedViaLimits(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	now := base.NewTime(time.Now())
	keeper := secrets.TestStringKeeper(t)

	depRepo := &depository.MockRepository{
		Depositories: []*model.Depository{
			{
				ID:            id.Depository("originator"),
				BankName:      "orig bank",
				Holder:        "orig",
				HolderType:    model.Individual,
				Type:          model.Checking,
				RoutingNumber: "121421212",
				Status:        model.DepositoryVerified,
				Metadata:      "metadata",
				Created:       now,
				Updated:       now,
				Keeper:        keeper,
			},
			{
				ID:            id.Depository("receiver"),
				BankName:      "receiver bank",
				Holder:        "receiver",
				HolderType:    model.Individual,
				Type:          model.Checking,
				RoutingNumber: "121421212",
				Status:        model.DepositoryVerified,
				Metadata:      "metadata",
				Created:       now,
				Updated:       now,
				Keeper:        keeper,
			},
		},
	}
	depRepo.Depositories[0].ReplaceAccountNumber("1321")
	depRepo.Depositories[1].ReplaceAccountNumber("323431")

	eventRepo := events.NewRepo(log.NewNopLogger(), db.DB)
	gatewayRepo := &gateways.MockRepository{
		Gateway: &model.Gateway{
			ID: model.GatewayID(base.ID()),
		},
	}
	recRepo := &receivers.MockRepository{
		Receivers: []*model.Receiver{
			{
				ID:                model.ReceiverID("receiver"),
				Email:             "foo@moov.io",
				DefaultDepository: id.Depository("receiver"),
				Status:            model.ReceiverVerified,
				Metadata:          "other",
				Created:           now,
				Updated:           now,
			},
		},
	}
	origRepo := &originators.MockRepository{
		Originators: []*model.Originator{
			{
				ID:                model.OriginatorID("originator"),
				DefaultDepository: id.Depository("originator"),
				Identification:    "id",
				Metadata:          "other",
				Created:           now,
				Updated:           now,
			},
		},
	}
	xferRepo := NewTransferRepo(log.NewNopLogger(), db.DB)

	router := CreateTestTransferRouter(depRepo, eventRepo, gatewayRepo, recRepo, origRepo, xferRepo)

	router.accountsClient = nil
	router.TransferRouter.accountsClient = nil

	// fake like we've sent money already, need a weird query...
	router.TransferRouter.transferLimitChecker.userTransferSumSQL = `select 34000.00 where "a" <> ? or "b" <> ?;`
	router.TransferRouter.transferLimitChecker.limits.CurrentDay, _ = model.NewAmount("USD", "35000.00")

	if total, err := router.TransferRouter.transferLimitChecker.userTransferSum(id.User("fake"), time.Now()); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("total=%.2f", total)
	}

	// Create our transfer
	amt, _ := model.NewAmount("USD", "18.61")
	request := &transferRequest{
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/transfers", &body)
	req.Header.Set("x-user-id", "test")
	router.createUserTransfers()(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP statu codes: %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "status=reviewable") {
		t.Errorf("unexpected error: %v", w.Body.String())
	}
}

func TestTransfers__read(t *testing.T) {
	amt, _ := model.NewAmount("USD", "27.12")
	request := transferRequest{
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
		Description:            "paycheck",
		StandardEntryClassCode: "PPD",
	}
	check := func(t *testing.T, req *transferRequest) {
		if req.Type != model.PushTransfer {
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
	keeper := secrets.TestStringKeeper(t)

	depRepo := &depository.MockRepository{
		Depositories: []*model.Depository{
			{
				ID:            id.Depository("originator"),
				BankName:      "orig bank",
				Holder:        "orig",
				HolderType:    model.Individual,
				Type:          model.Checking,
				RoutingNumber: "121421212",
				Status:        model.DepositoryVerified,
				Metadata:      "metadata",
				Created:       now,
				Updated:       now,
				Keeper:        keeper,
			},
			{
				ID:            id.Depository("receiver"),
				BankName:      "receiver bank",
				Holder:        "receiver",
				HolderType:    model.Individual,
				Type:          model.Checking,
				RoutingNumber: "121421212",
				Status:        model.DepositoryVerified,
				Metadata:      "metadata",
				Created:       now,
				Updated:       now,
				Keeper:        keeper,
			},
		},
	}
	depRepo.Depositories[0].ReplaceAccountNumber("1321")
	depRepo.Depositories[1].ReplaceAccountNumber("323431")

	eventRepo := events.NewRepo(logger, db.DB)
	gatewayRepo := &gateways.MockRepository{
		Gateway: &model.Gateway{
			ID: model.GatewayID(base.ID()),
		},
	}
	recRepo := &receivers.MockRepository{
		Receivers: []*model.Receiver{
			{
				ID:                model.ReceiverID("receiver"),
				Email:             "foo@moov.io",
				DefaultDepository: id.Depository("receiver"),
				Status:            model.ReceiverVerified,
				Metadata:          "other",
				Created:           now,
				Updated:           now,
			},
		},
	}
	origRepo := &originators.MockRepository{
		Originators: []*model.Originator{
			{
				ID:                model.OriginatorID("originator"),
				DefaultDepository: id.Depository("originator"),
				Identification:    "id",
				Metadata:          "other",
				Created:           now,
				Updated:           now,
			},
		},
	}
	repo := &SQLRepo{db.DB, log.NewNopLogger()}

	amt, _ := model.NewAmount("USD", "18.61")
	request := &transferRequest{
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
		PPDDetail: &model.PPDDetail{
			PaymentInformation: "payment",
		},
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	router := CreateTestTransferRouter(depRepo, eventRepo, gatewayRepo, recRepo, origRepo, repo)

	router.accountsClient = nil
	router.TransferRouter.accountsClient = nil

	req, _ := http.NewRequest("POST", "/transfers", &body)
	req.Header.Set("x-user-id", "test")
	router.createUserTransfers()(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus HTTP statu codes: %d: %s", w.Code, w.Body.String())
	}

	var xfer model.Transfer
	if err := json.NewDecoder(w.Body).Decode(&xfer); err != nil {
		t.Fatal(err)
	}
	if tt, err := repo.getUserTransfer(xfer.ID, id.User("test")); tt == nil || tt.ID == "" || err != nil {
		t.Fatalf("missing Transfer=%#v error=%v", tt, err)
	}
}

func TestTransfers__idempotency(t *testing.T) {
	// The repositories aren't used, aka idempotency check needs to be first.
	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)

	req := httptest.NewRequest("POST", "/transfers", nil)
	req.Header.Set("x-idempotency-key", "key")
	req.Header.Set("x-user-id", "user")

	// mark the key as seen
	if seen := route.IdempotentRecorder.SeenBefore("key"); seen {
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

	repo := &SQLRepo{db.DB, log.NewNopLogger()}

	amt, _ := model.NewAmount("USD", "18.61")
	userID := id.User(base.ID())
	req := &transferRequest{
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
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
	r.Header.Set("x-user-id", userID.String())

	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, nil, repo)

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)

	}

	var transfer model.Transfer
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
	xferRouter.transferRepo = &MockRepository{Err: errors.New("bad error")}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__readTransferFilterParams(t *testing.T) {
	u, _ := url.Parse("http://localhost:8082/transfers?startDate=2020-04-06&limit=10&status=failed")
	req := &http.Request{URL: u}
	params := readTransferFilterParams(req)

	if params.StartDate.Format(util.YYMMDDTimeFormat) != "2020-04-06" {
		t.Errorf("unexpected StartDate: %v", params.StartDate)
	}
	if !params.EndDate.After(time.Now()) {
		t.Errorf("unexpected EndDate: %v", params.EndDate)
	}
	if params.Status != model.TransferFailed {
		t.Errorf("expected status: %q", params.Status)
	}
	if params.Limit != 10 {
		t.Errorf("unexpected limit: %d", params.Limit)
	}
	if params.Offset != 0 {
		t.Errorf("unexpected offset: %d", params.Offset)
	}
}

func TestTransfers__getUserTransfers(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLRepo{db.DB, log.NewNopLogger()}

	amt, _ := model.NewAmount("USD", "12.42")
	userID := id.User(base.ID())
	req := &transferRequest{
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}

	if _, err := repo.createUserTransfers(userID, []*transferRequest{req}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/transfers", nil)
	r.Header.Set("x-user-id", userID.String())

	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, nil, repo)

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}

	var transfers []*model.Transfer
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
	xferRouter.transferRepo = &MockRepository{Err: errors.New("bad error")}

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

	repo := &SQLRepo{db.DB, log.NewNopLogger()}

	amt, _ := model.NewAmount("USD", "12.42")
	userID := id.User(base.ID())
	req := &transferRequest{
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
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
	r.Header.Set("x-user-id", userID.String())

	xferRouter := CreateTestTransferRouter(nil, nil, nil, nil, nil, repo)

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d: %s", w.Code, w.Body.String())
	}

	// have our repository error and verify we get non-200's
	xferRouter.transferRepo = &MockRepository{Err: errors.New("bad error")}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__writeResponse(t *testing.T) {
	w := httptest.NewRecorder()

	amt, _ := model.NewAmount("USD", "12.42")

	var transfers []*model.Transfer
	transfers = append(transfers, transferRequest{
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}.asTransfer(base.ID()))

	// Respond with one transfer, shouldn't be wrapped in an array
	writeResponse(log.NewNopLogger(), w, 1, transfers)
	w.Flush()

	var singleResponse model.Transfer
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

	var pluralResponse []model.Transfer
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

func TestTransfers__HTTPGetNoUserID(t *testing.T) {
	xfer := CreateTestTransferRouter(nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()

	xfer.RegisterRoutes(router)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/transfers", nil)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__HTTPCreateNoUserID(t *testing.T) {
	xfer := CreateTestTransferRouter(nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()

	xfer.RegisterRoutes(router)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/transfers", nil)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__HTTPCreateBatchNoUserID(t *testing.T) {
	xfer := CreateTestTransferRouter(nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()

	xfer.RegisterRoutes(router)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/transfers/batch", nil)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__HTTPDeleteNoUserID(t *testing.T) {
	xfer := CreateTestTransferRouter(nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()

	xfer.RegisterRoutes(router)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/transfers/foo", nil)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__HTTPValidateNoUserID(t *testing.T) {
	xfer := CreateTestTransferRouter(nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()

	xfer.RegisterRoutes(router)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/transfers/foo/failed", nil)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__HTTPFilesNoUserID(t *testing.T) {
	xfer := CreateTestTransferRouter(nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()

	xfer.RegisterRoutes(router)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/transfers/foo/files", nil)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__createWithCustomerError(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	logger := log.NewNopLogger()
	now := base.NewTime(time.Now())
	keeper := secrets.TestStringKeeper(t)

	depRepo := &depository.MockRepository{
		Depositories: []*model.Depository{
			{
				ID:            id.Depository("originator"),
				BankName:      "orig bank",
				Holder:        "orig",
				HolderType:    model.Individual,
				Type:          model.Checking,
				RoutingNumber: "121421212",
				Status:        model.DepositoryVerified,
				Metadata:      "metadata",
				Created:       now,
				Updated:       now,
				Keeper:        keeper,
			},
			{
				ID:            id.Depository("receiver"),
				BankName:      "receiver bank",
				Holder:        "receiver",
				HolderType:    model.Individual,
				Type:          model.Checking,
				RoutingNumber: "121421212",
				Status:        model.DepositoryVerified,
				Metadata:      "metadata",
				Created:       now,
				Updated:       now,
				Keeper:        keeper,
			},
		},
	}
	depRepo.Depositories[0].ReplaceAccountNumber("1321")
	depRepo.Depositories[1].ReplaceAccountNumber("323431")

	eventRepo := events.NewRepo(logger, db.DB)
	gatewayRepo := &gateways.MockRepository{
		Gateway: &model.Gateway{
			ID: model.GatewayID(base.ID()),
		},
	}
	recRepo := &receivers.MockRepository{
		Receivers: []*model.Receiver{
			{
				ID:                model.ReceiverID("receiver"),
				Email:             "foo@moov.io",
				DefaultDepository: id.Depository("receiver"),
				Status:            model.ReceiverVerified,
				Metadata:          "other",
				Created:           now,
				Updated:           now,
			},
		},
	}
	origRepo := &originators.MockRepository{
		Originators: []*model.Originator{
			{
				ID:                model.OriginatorID("originator"),
				DefaultDepository: id.Depository("originator"),
				Identification:    "id",
				Metadata:          "other",
				Created:           now,
				Updated:           now,
			},
		},
	}
	repo := &SQLRepo{db.DB, log.NewNopLogger()}

	amt, _ := model.NewAmount("USD", "18.61")
	request := &transferRequest{
		Type:                   model.PullTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
		PPDDetail: &model.PPDDetail{
			PaymentInformation: "payment",
		},
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	router := CreateTestTransferRouter(depRepo, eventRepo, gatewayRepo, recRepo, origRepo, repo)

	router.accountsClient = nil
	router.TransferRouter.accountsClient = nil
	router.customersClient = &customers.TestClient{
		Err: errors.New("createWithCustomerError"),
	}

	req, _ := http.NewRequest("POST", "/transfers", &body)
	req.Header.Set("x-user-id", "test")
	router.createUserTransfers()(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP statu codes: %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "verifyCustomerStatuses: originator: createWithCustomerError") {
		t.Errorf("unexpected error: %v", w.Body.String())
	}
}

func TestTransferObjects(t *testing.T) {
	userID := id.User(base.ID())

	depID := id.Depository(base.ID())
	origID := model.OriginatorID(base.ID())
	recID := model.ReceiverID(base.ID())

	transferRepo := &MockRepository{}
	router := setupTestRouter(t, transferRepo)
	router.receiverRepo.Receivers = []*model.Receiver{
		{
			ID:                model.ReceiverID(base.ID()),
			Email:             "test@moov.io",
			DefaultDepository: id.Depository(base.ID()),
			Status:            model.ReceiverVerified,
		},
	}
	router.depositoryRepo.Depositories = nil

	rec, recDep, orig, origDep, err := router.getTransferObjects(userID, origID, depID, recID, depID)
	if err == nil || !strings.Contains(err.Error(), "receiver depository not found") {
		t.Errorf("expected error: %v", err)
	}
	if rec != nil || recDep != nil || orig != nil || origDep != nil {
		t.Errorf("receciver=%#v", rec)
		t.Errorf("receciver depository=%#v", recDep)
		t.Errorf("originator=%#v", orig)
		t.Errorf("originator depository=%#v", origDep)
	}
}

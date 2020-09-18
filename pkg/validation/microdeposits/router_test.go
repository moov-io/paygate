// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"
	moovcustomers "github.com/moov-io/customers/pkg/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers"
	"github.com/moov-io/paygate/pkg/customers/accounts"
	"github.com/moov-io/paygate/pkg/testclient"
	"github.com/moov-io/paygate/pkg/transfers"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"

	"github.com/gorilla/mux"
)

var (
	sourceCustomerID, sourceAccountID           = base.ID(), base.ID()
	destinationCustomerID, destinationAccountID = base.ID(), base.ID()

	mockTransferRepo = &transfers.MockRepository{
		Transfers: []*client.Transfer{
			{
				TransferID: base.ID(),
				Amount: client.Amount{
					Currency: "USD",
					Value:    1244,
				},
				Source: client.Source{
					CustomerID: sourceCustomerID,
					AccountID:  sourceAccountID,
				},
				Destination: client.Destination{
					CustomerID: destinationCustomerID,
					AccountID:  destinationAccountID,
				},
				Description: "test transfer",
				Status:      client.PENDING,
				Created:     time.Now(),
			},
		},
	}

	fakePublisher = pipeline.NewMockPublisher()

	mockStrategy = &fundflow.MockStrategy{}

	mockDecryptor = &accounts.MockDecryptor{Number: "12345"}
)

func mockCustomersClient() *customers.MockClient {
	client := &customers.MockClient{
		Accounts: make(map[string]*moovcustomers.Account),
		Customers: []*moovcustomers.Customer{
			{
				CustomerID: sourceCustomerID,
				FirstName:  "John",
				LastName:   "Doe",
				Email:      "john.doe@example.com",
				Status:     moovcustomers.VERIFIED,
			},
			{
				CustomerID: destinationCustomerID,
				FirstName:  "John",
				LastName:   "Doe",
				Email:      "john.doe@example.com",
				Status:     moovcustomers.RECEIVE_ONLY,
			},
		},
	}
	client.Accounts[sourceAccountID] = &moovcustomers.Account{
		AccountID:           sourceAccountID,
		MaskedAccountNumber: "****34",
		RoutingNumber:       "987654320",
		Status:              moovcustomers.VALIDATED,
		Type:                moovcustomers.CHECKING,
	}
	client.Accounts[destinationAccountID] = &moovcustomers.Account{
		AccountID:           destinationAccountID,
		MaskedAccountNumber: "****34",
		RoutingNumber:       "123456780",
		Status:              moovcustomers.NONE,
		Type:                moovcustomers.CHECKING,
	}
	return client
}

func mockMicroDeposit() *client.MicroDeposits {
	return &client.MicroDeposits{
		MicroDepositID: base.ID(),
		TransferIDs:    []string{base.ID(), base.ID()},
		Destination: client.Destination{
			CustomerID: destinationCustomerID,
			AccountID:  destinationAccountID,
		},
		Amounts: []client.Amount{
			{Currency: "USD", Value: 2},
			{Currency: "USD", Value: 5},
		},
		Status:  client.PENDING,
		Created: time.Now(),
	}
}

func mockConfig() *config.Config {
	cfg := config.Empty()
	cfg.Validation = config.Validation{
		MicroDeposits: &config.MicroDeposits{
			Source: config.Source{
				CustomerID: sourceCustomerID,
				AccountID:  sourceAccountID,
			},
		},
	}
	return cfg
}

func TestRouter__NotImplemented(t *testing.T) {
	cfg := config.Empty()
	customersClient := mockCustomersClient()

	repo := &mockRepository{
		Micro: mockMicroDeposit(),
	}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	req := httptest.NewRequest("GET", fmt.Sprintf("/micro-deposits/%s", base.ID()), nil)
	req.Header.Set("X-Namespace", base.ID())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus HTTP status %d: %v", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "micro-deposits are disabled") {
		t.Errorf("unexpected error: %v", w.Body.String())
	}
}

func TestRouter__InitiateMicroDeposits(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()

	repo := &mockRepository{
		Micro: mockMicroDeposit(),
	}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	namespace := base.ID()
	micro, resp, err := c.ValidationApi.InitiateMicroDeposits(context.TODO(), namespace, client.CreateMicroDeposits{
		Destination: client.Destination{
			CustomerID: destinationCustomerID,
			AccountID:  destinationAccountID,
		},
	})
	if err != nil {
		t.Errorf("%#v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	if micro.MicroDepositID == "" {
		t.Error("missing MicroDeposit")
	}
}

func TestRouter__InitiateMicroDepositsErr(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()
	repo := &mockRepository{Err: errors.New("bad request")}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	namespace := base.ID()
	_, resp, err := c.ValidationApi.InitiateMicroDeposits(context.TODO(), namespace, client.CreateMicroDeposits{})
	if err == nil {
		t.Fatal("expected error")
	}
	resp.Body.Close()
}

func TestRouter__GetMicroDeposits(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()

	repo := &mockRepository{
		Micro: mockMicroDeposit(),
	}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	namespace := base.ID()
	micro, resp, err := c.ValidationApi.GetMicroDeposits(context.TODO(), repo.Micro.MicroDepositID, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if micro.MicroDepositID == "" {
		t.Error("missing MicroDeposit")
	}
}

func TestRouter__GetMicroDepositsEmpty(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()

	repo := &mockRepository{}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	namespace := base.ID()
	micro, resp, err := c.ValidationApi.GetMicroDeposits(context.TODO(), base.ID(), namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if micro.MicroDepositID != "" {
		t.Errorf("unexpected MicroDeposit: %#v", micro)
	}
}

func TestRouter__GetMicroDepositsErr(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()

	repo := &mockRepository{Err: errors.New("bad error")}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	namespace := base.ID()
	_, resp, err := c.ValidationApi.GetMicroDeposits(context.TODO(), base.ID(), namespace)
	if err == nil {
		t.Fatal("expected error")
	}
	resp.Body.Close()
}

func TestRouter__GetAccountMicroDeposits(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()

	repo := &mockRepository{
		Micro: mockMicroDeposit(),
	}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	namespace := base.ID()
	micro, resp, err := c.ValidationApi.GetAccountMicroDeposits(context.TODO(), repo.Micro.Destination.AccountID, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if micro.MicroDepositID == "" {
		t.Error("missing MicroDeposit")
	}
}

func TestRouter__GetAccountMicroDepositsEmpty(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()

	repo := &mockRepository{}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	namespace := base.ID()
	micro, resp, err := c.ValidationApi.GetAccountMicroDeposits(context.TODO(), base.ID(), namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if micro.MicroDepositID != "" {
		t.Errorf("unexpected MicroDeposit: %#v", micro)
	}
}

func TestRouter__GetAccountMicroDepositsErr(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()

	repo := &mockRepository{Err: errors.New("bad error")}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	namespace := base.ID()
	_, resp, err := c.ValidationApi.GetAccountMicroDeposits(context.TODO(), base.ID(), namespace)
	if err == nil {
		t.Fatal("expected error")
	}
	resp.Body.Close()
}

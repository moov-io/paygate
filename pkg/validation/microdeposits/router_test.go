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
	moovcustomers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers"
	"github.com/moov-io/paygate/pkg/customers/accounts"
	"github.com/moov-io/paygate/pkg/tenants"
	"github.com/moov-io/paygate/pkg/testclient"
	"github.com/moov-io/paygate/pkg/transfers"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"

	"github.com/gorilla/mux"
)

var (
	customerID, accountID = base.ID(), base.ID()
	mockTransferRepo      = &transfers.MockRepository{
		Transfers: []*client.Transfer{
			{
				TransferID: base.ID(),
				Amount:     "USD 12.44",
				Source: client.Source{
					CustomerID: customerID,
					AccountID:  accountID,
				},
				Destination: client.Destination{
					CustomerID: base.ID(),
					AccountID:  base.ID(),
				},
				Description: "test transfer",
				Status:      client.PENDING,
				Created:     time.Now(),
			},
		},
	}

	tenantRepo = &tenants.MockRepository{}

	fakePublisher = &pipeline.MockPublisher{}

	mockStrategy = &fundflow.MockStrategy{}

	mockDecryptor = &accounts.MockDecryptor{Number: "12345"}
)

func mockCustomersClient() *customers.MockClient {
	return &customers.MockClient{
		Account: &moovcustomers.Account{
			AccountID:           base.ID(),
			MaskedAccountNumber: "****34",
			RoutingNumber:       "987654320",
			Status:              moovcustomers.VALIDATED,
			Type:                moovcustomers.CHECKING,
		},
		Customer: &moovcustomers.Customer{
			CustomerID: base.ID(),
			FirstName:  "John",
			LastName:   "Doe",
			Email:      "john.doe@example.com",
			Status:     moovcustomers.VERIFIED,
		},
	}
}

func mockMicroDeposit() *client.MicroDeposits {
	return &client.MicroDeposits{
		MicroDepositID: base.ID(),
		TransferIDs:    []string{base.ID(), base.ID()},
		Destination: client.Destination{
			CustomerID: base.ID(),
			AccountID:  base.ID(),
		},
		Amounts: []string{"USD 0.02", "USD 0.05"},
		Status:  client.PENDING,
		Created: time.Now(),
	}
}

func mockConfig() *config.Config {
	cfg := config.Empty()
	cfg.Validation = config.Validation{
		MicroDeposits: &config.MicroDeposits{
			Source: config.Source{
				CustomerID: customerID,
				AccountID:  accountID,
			},
			Description: "account validation",
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
	router := NewRouter(cfg, repo, mockTransferRepo, tenantRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	req := httptest.NewRequest("GET", fmt.Sprintf("/micro-deposits/%s", base.ID()), nil)
	req.Header.Set("x-user-id", base.ID())

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
	router := NewRouter(cfg, repo, mockTransferRepo, tenantRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	userID := base.ID()
	micro, resp, err := c.ValidationApi.InitiateMicroDeposits(context.TODO(), userID, client.CreateMicroDeposits{
		Destination: client.Destination{
			CustomerID: customerID,
			AccountID:  accountID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if micro.MicroDepositID == "" {
		t.Error("missing MicroDeposit")
	}
}

func TestRouter__InitiateMicroDepositsErr(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()
	repo := &mockRepository{Err: errors.New("bad request")}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, tenantRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	userID := base.ID()
	_, resp, err := c.ValidationApi.InitiateMicroDeposits(context.TODO(), userID, client.CreateMicroDeposits{})
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
	router := NewRouter(cfg, repo, mockTransferRepo, tenantRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	userID := base.ID()
	micro, resp, err := c.ValidationApi.GetMicroDeposits(context.TODO(), repo.Micro.MicroDepositID, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if micro.MicroDepositID == "" {
		t.Error("missing MicroDeposit")
	}
}

func TestRouter__GetMicroDepositsErr(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()

	repo := &mockRepository{Err: errors.New("bad error")}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, tenantRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	userID := base.ID()
	_, resp, err := c.ValidationApi.GetMicroDeposits(context.TODO(), base.ID(), userID)
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
	router := NewRouter(cfg, repo, mockTransferRepo, tenantRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	userID := base.ID()
	micro, resp, err := c.ValidationApi.GetAccountMicroDeposits(context.TODO(), repo.Micro.Destination.AccountID, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if micro.MicroDepositID == "" {
		t.Error("missing MicroDeposit")
	}
}

func TestRouter__GetAccountMicroDepositsErr(t *testing.T) {
	cfg := mockConfig()
	customersClient := mockCustomersClient()

	repo := &mockRepository{Err: errors.New("bad error")}

	r := mux.NewRouter()
	router := NewRouter(cfg, repo, mockTransferRepo, tenantRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	userID := base.ID()
	_, resp, err := c.ValidationApi.GetAccountMicroDeposits(context.TODO(), base.ID(), userID)
	if err == nil {
		t.Fatal("expected error")
	}
	resp.Body.Close()
}

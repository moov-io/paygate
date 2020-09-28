// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"
	moovcustomers "github.com/moov-io/customers/pkg/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers"
	"github.com/moov-io/paygate/pkg/customers/accounts"
	"github.com/moov-io/paygate/pkg/namespace"
	"github.com/moov-io/paygate/pkg/testclient"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"
	"github.com/moov-io/paygate/pkg/util"

	"github.com/gorilla/mux"
)

var (
	sourceCustomerID, destinationCustomerID = base.ID(), base.ID()
	sourceAccountID, destinationAccountID   = base.ID(), base.ID()

	repoWithTransfer = &MockRepository{
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
				TraceNumbers: []string{
					"Trace123",
					"Trace124",
				},
			},
		},
	}

	namespaceRepo = &namespace.MockRepository{}

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
				FirstName:  "Jane",
				LastName:   "Doe",
				Email:      "jane.doe@example.com",
				Status:     moovcustomers.VERIFIED,
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
		RoutingNumber:       "987654320",
		Status:              moovcustomers.VALIDATED,
		Type:                moovcustomers.CHECKING,
	}
	return client
}

func TestTransfers__readTransferFilterParams(t *testing.T) {
	u, _ := url.Parse("http://localhost:8082/transfers?startDate=2020-04-06&count=10&status=failed")
	req := &http.Request{URL: u}
	params := readTransferFilterParams(req)

	if params.StartDate.Format(util.YYMMDDTimeFormat) != "2020-04-06" {
		t.Errorf("unexpected StartDate: %v", params.StartDate)
	}
	if !params.EndDate.After(time.Now()) {
		t.Errorf("unexpected EndDate: %v", params.EndDate)
	}
	if params.Status != client.FAILED {
		t.Errorf("expected status: %q", params.Status)
	}
	if params.Count != 10 {
		t.Errorf("unexpected count: %d", params.Count)
	}
	if params.Skip != 0 {
		t.Errorf("unexpected skip: %d", params.Skip)
	}
}

func TestRouter__getUserTransfers(t *testing.T) {
	customersClient := mockCustomersClient()

	r := mux.NewRouter()
	router := NewRouter(config.Empty(), repoWithTransfer, namespaceRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	xfers, resp, err := c.TransfersApi.GetTransfers(context.TODO(), "namespace", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if n := len(xfers); n != 1 {
		t.Errorf("got %d transfers: %#v", n, xfers)
	}
}

func TestRouter__createUserTransfer(t *testing.T) {
	customersClient := mockCustomersClient()

	r := mux.NewRouter()
	router := NewRouter(config.Empty(), repoWithTransfer, namespaceRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	opts := client.CreateTransfer{
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
		SameDay:     true,
	}
	xfer, resp, err := c.TransfersApi.AddTransfer(context.TODO(), "namespace", opts, nil)
	if err != nil {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("error=%v \n body=%s", err, string(bs))
	}
	defer resp.Body.Close()

	if xfer.TransferID == "" {
		t.Errorf("missing Transfer=%#v", xfer)
	}
}

func TestRouter__createUserTransfersInvalidAmount(t *testing.T) {
	customersClient := mockCustomersClient()

	r := mux.NewRouter()
	router := NewRouter(config.Empty(), repoWithTransfer, namespaceRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	opts := client.CreateTransfer{
		Amount: client.Amount{
			Currency: "USD",
			Value:    -1,
		},
	}
	xfer, resp, err := c.TransfersApi.AddTransfer(context.TODO(), "namespace", opts, nil)
	if err == nil {
		t.Error("expected error")
	}
	defer resp.Body.Close()

	if xfer.TransferID != "" {
		t.Errorf("unexpected transfer: %#v", xfer)
	}
}

func TestRouter__createUserTransferMissingFundflowStrategy(t *testing.T) {
	customersClient := mockCustomersClient()

	r := mux.NewRouter()
	router := NewRouter(config.Empty(), repoWithTransfer, namespaceRepo, customersClient, mockDecryptor, nil, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	opts := client.CreateTransfer{
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
		SameDay:     true,
	}
	_, resp, err := c.TransfersApi.AddTransfer(context.TODO(), "namespace", opts, nil)
	if err == nil {
		t.Error("expected error")
	} else {
		if e, ok := err.(client.GenericOpenAPIError); ok {
			if !strings.Contains(fmt.Sprintf("%#v", e.Model()), "no fundflow strategy configured") {
				t.Fatalf("unexpected error: %#v", e.Model())
			}
		} else {
			t.Fatalf("unexpected error: %#v", err)
		}
	}
	defer resp.Body.Close()
}

func TestRouter__MissingSource(t *testing.T) {
	customersClient := mockCustomersClient()

	r := mux.NewRouter()
	router := NewRouter(config.Empty(), repoWithTransfer, namespaceRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	opts := client.CreateTransfer{
		Amount: client.Amount{
			Currency: "USD",
			Value:    1254,
		},
		Source: client.Source{
			AccountID: base.ID(), // missing CustomerID
		},
	}
	xfer, resp, err := c.TransfersApi.AddTransfer(context.TODO(), "namespace", opts, nil)
	if err == nil {
		t.Error("expected error")
	}
	defer resp.Body.Close()

	if xfer.TransferID != "" {
		t.Errorf("unexpected transfer: %#v", xfer)
	}
}

func TestRouter__MissingDestination(t *testing.T) {
	customersClient := mockCustomersClient()

	r := mux.NewRouter()
	router := NewRouter(config.Empty(), repoWithTransfer, namespaceRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	opts := client.CreateTransfer{
		Amount: client.Amount{
			Currency: "USD",
			Value:    1254,
		},
		Source: client.Source{
			CustomerID: sourceCustomerID,
			AccountID:  sourceAccountID,
		},
		Destination: client.Destination{
			CustomerID: base.ID(), // missing AccountID
		},
	}
	xfer, resp, err := c.TransfersApi.AddTransfer(context.TODO(), "namespace", opts, nil)
	if err == nil {
		t.Error("expected error")
	}
	defer resp.Body.Close()

	if xfer.TransferID != "" {
		t.Errorf("unexpected transfer: %#v", xfer)
	}
}

func TestRouter__validateAmount(t *testing.T) {
	amt := client.Amount{
		Currency: "USD",
		Value:    112,
	}
	if err := validateAmount(amt); err != nil {
		t.Errorf("expected no error: %v", err)
	}

	// invalid currency
	amt.Currency = "ZZZ"
	err := validateAmount(amt)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unexpected currency") {
		t.Errorf("unexpected error: %v", err)
	}

	// invalid amount
	amt.Value = 0
	err = validateAmount(amt)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid amount") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRouter__getUserTransfer(t *testing.T) {
	customersClient := mockCustomersClient()

	r := mux.NewRouter()
	router := NewRouter(config.Empty(), repoWithTransfer, namespaceRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	xfer, resp, err := c.TransfersApi.GetTransferByID(context.TODO(), "transferID", "namespace", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if xfer.TransferID == "" {
		t.Errorf("missing Transfer=%#v", xfer)
	}
}

func TestRouter__deleteUserTransfer(t *testing.T) {
	customersClient := mockCustomersClient()

	r := mux.NewRouter()
	router := NewRouter(config.Empty(), repoWithTransfer, namespaceRepo, customersClient, mockDecryptor, mockStrategy, fakePublisher)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	resp, err := c.TransfersApi.DeleteTransferByID(context.TODO(), "transferID", "namespace", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

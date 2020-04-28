// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/testclient"
	"github.com/moov-io/paygate/pkg/transfers/offload"
)

var (
	repoWithTransfer = &MockRepository{
		Transfers: []*client.Transfer{
			{
				TransferID: base.ID(),
				Amount:     "USD 12.44",
				Source: client.Source{
					CustomerID: base.ID(),
					AccountID:  base.ID(),
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

	fakeOffloader = &offload.MockOffloader{}
)

func TestRouter__getUserTransfers(t *testing.T) {
	r := mux.NewRouter()
	router := NewRouter(log.NewNopLogger(), repoWithTransfer, fakeOffloader)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	xfers, resp, err := c.TransfersApi.GetTransfers(context.TODO(), "userID", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if n := len(xfers); n != 1 {
		t.Errorf("got %d transfers: %#v", n, xfers)
	}
}

func TestRouter__createUserTransfer(t *testing.T) {
	r := mux.NewRouter()
	router := NewRouter(log.NewNopLogger(), repoWithTransfer, fakeOffloader)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	opts := client.CreateTransfer{
		Amount: "USD 12.44",
		Source: client.Source{
			CustomerID: base.ID(),
			AccountID:  base.ID(),
		},
		Destination: client.Destination{
			CustomerID: base.ID(),
			AccountID:  base.ID(),
		},
		Description: "test transfer",
		SameDay:     true,
	}
	xfer, resp, err := c.TransfersApi.AddTransfer(context.TODO(), "userID", opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if xfer.TransferID == "" {
		t.Errorf("missing Transfer=%#v", xfer)
	}
}

func TestRouter__getUserTransfer(t *testing.T) {
	r := mux.NewRouter()
	router := NewRouter(log.NewNopLogger(), repoWithTransfer, fakeOffloader)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	xfer, resp, err := c.TransfersApi.GetTransferByID(context.TODO(), "transferID", "userID", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if xfer.TransferID == "" {
		t.Errorf("missing Transfer=%#v", xfer)
	}
}

func TestRouter__deleteUserTransfer(t *testing.T) {
	r := mux.NewRouter()
	router := NewRouter(log.NewNopLogger(), repoWithTransfer, fakeOffloader)
	router.RegisterRoutes(r)

	c := testclient.New(t, r)

	resp, err := c.TransfersApi.DeleteTransferByID(context.TODO(), "transferID", "userID", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

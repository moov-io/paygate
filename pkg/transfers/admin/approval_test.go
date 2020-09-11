// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/admin"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/testclient"
	"github.com/moov-io/paygate/pkg/transfers"

	"github.com/go-kit/kit/log"
)

func TestAdmin__updateTransferStatus(t *testing.T) {
	repo := &transfers.MockRepository{
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

	svc, c := testclient.Admin(t)
	RegisterRoutes(log.NewNopLogger(), svc, repo)

	req := admin.UpdateTransferStatus{
		Status: admin.CANCELED,
	}
	resp, err := c.TransfersApi.UpdateTransferStatus(context.TODO(), "transferID", "namespace", req, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || err != nil {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
		t.Fatal(err)
	}

}

func TestAdmin__validStatusTransistion(t *testing.T) {
	transferID := base.ID()

	// Reviewable into pending or cancel
	if err := validStatusTransistion(transferID, client.REVIEWABLE, client.CANCELED); err != nil {
		t.Error(err)
	}
	if err := validStatusTransistion(transferID, client.REVIEWABLE, client.PENDING); err != nil {
		t.Error(err)
	}
	// Reviewable into an unaccepted status
	if err := validStatusTransistion(transferID, client.REVIEWABLE, client.PROCESSED); err == nil {
		t.Error("expected error")
	}

	// Pending to Canceled
	if err := validStatusTransistion(transferID, client.PENDING, client.CANCELED); err != nil {
		t.Error(err)
	}
	if err := validStatusTransistion(transferID, client.REVIEWABLE, client.PROCESSED); err == nil {
		t.Error("expected error")
	}

	// Pending to Reviewable
	if err := validStatusTransistion(transferID, client.PENDING, client.REVIEWABLE); err != nil {
		t.Error(err)
	}
}

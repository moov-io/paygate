// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"fmt"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	customers "github.com/moov-io/customers/pkg/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers/accounts"
	"github.com/moov-io/base/database"
	"github.com/moov-io/paygate/pkg/transfers"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"
)

func between(amt client.Amount) error {
	if amt.Value >= 1 && amt.Value <= 25 {
		return nil
	}
	return fmt.Errorf("invalid amount %q", amt)
}

func TestAmountConditions(t *testing.T) {
	if err := between(client.Amount{Value: 10}); err != nil {
		t.Error(err)
	}
	if err := between(client.Amount{Value: 24}); err != nil {
		t.Error(err)
	}

	if err := between(client.Amount{Value: 0}); err == nil {
		t.Error("expected error")
	}
	if err := between(client.Amount{Value: 26}); err == nil {
		t.Error("expected error")
	}

	if err := between(client.Amount{}); err == nil {
		t.Error("expected error")
	}
	if err := between(client.Amount{Value: -1}); err == nil {
		t.Error("expected error")
	}
}

func TestAmounts(t *testing.T) {
	amt1, amt2 := getMicroDepositAmounts()
	if err := between(amt1); err != nil {
		t.Error(err)
	}
	if err := between(amt2); err != nil {
		t.Error(err)
	}
}

func TestMicroDeposits__createMicroDeposits(t *testing.T) {
	cfg := mockConfig()
	cfg.ODFI.RoutingNumber = "123456780"
	organization := base.ID()

	db := database.CreateTestSQLiteDB(t)
	t.Cleanup(func() { db.Close() })

	src, dest := createTestSource(cfg.ODFI), createTestDestination()

	repo := transfers.NewRepo(db.DB)
	decryptor := &accounts.MockDecryptor{
		Number: "12345",
	}
	pub := pipeline.NewMockPublisher()
	strategy := fundflow.NewFirstPerson(cfg.Logger, cfg.ODFI)

	companyID := "MoovZZZZZZ"
	micro, err := createMicroDeposits(*cfg.Validation.MicroDeposits, organization, companyID, src, dest, repo, decryptor, strategy, pub)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(micro.TransferIDs); n != 3 {
		t.Fatalf("got %d micro-deposit transfers: %#v", n, micro)
	}

	for i := range micro.TransferIDs {
		xfer, err := repo.GetTransfer(micro.TransferIDs[i])
		if xfer == nil || err != nil {
			t.Fatalf("transferID=%s error=%v", micro.TransferIDs[i], err)
		}
		if xfer, ok := pub.Xfers[micro.TransferIDs[i]]; !ok {
			t.Fatalf("missing transferID=%s", micro.TransferIDs[i])
		} else {
			if len(xfer.File.Batches) != 1 {
				t.Errorf("batches: %#v", xfer.File.Batches)
			}
			entries := xfer.File.Batches[0].GetEntries()
			if len(entries) != 1 {
				t.Errorf("entries: %#v", entries)
			}

			if testing.Verbose() {
				fmt.Printf("\n%#v\n", xfer.File.Header)
				fmt.Printf("   %#v\n", xfer.File.Batches[0].GetHeader())
				fmt.Printf("      %#v\n\n", entries[0])
			}

			switch {
			case entries[0].RDFIIdentification != "98765432":
				t.Errorf("unexpected RDFI for EntryDetail: %#v", entries[0])

			case entries[0].RDFIIdentification == "98765432":
				if entries[0].DFIAccountNumber != "12345" {
					t.Errorf("entries[0].DFIAccountNumber=%q", entries[0].DFIAccountNumber)
				}
				if entries[0].TransactionCode != ach.CheckingCredit {
					if entries[0].IndividualName != "Jon Doe" {
						t.Errorf("entries[0].IndividualName=%q", entries[0].IndividualName)
					}
				}
				if entries[0].TransactionCode != ach.SavingsDebit {
					if entries[0].IndividualName != "Jon Doe" {
						t.Errorf("entries[0].IndividualName=%q", entries[0].IndividualName)
					}
				}

			default:
				t.Errorf("entries[0].RDFIIdentification=%s", entries[0].RDFIIdentification)
			}
		}
	}
}

func createTestSource(odfi config.ODFI) fundflow.Source {
	return fundflow.Source{
		Customer: customers.Customer{
			CustomerID: "src-customer",
			FirstName:  "Jane",
			LastName:   "Doe",
			Status:     customers.CUSTOMERSTATUS_VERIFIED,
		},
		Account: customers.Account{
			AccountID:     "src-account",
			RoutingNumber: odfi.RoutingNumber,
			Type:          customers.ACCOUNTTYPE_CHECKING,
		},
	}
}

func createTestDestination() fundflow.Destination {
	return fundflow.Destination{
		Customer: customers.Customer{
			CustomerID: "dest-customer",
			FirstName:  "Jon",
			LastName:   "Doe",
			Status:     customers.CUSTOMERSTATUS_VERIFIED,
		},
		Account: customers.Account{
			AccountID:     "dest-account",
			RoutingNumber: "987654320",
			Type:          customers.ACCOUNTTYPE_SAVINGS,
		},
		AccountNumber: "12345",
	}
}

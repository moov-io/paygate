// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"fmt"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	customers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers/accounts"
	"github.com/moov-io/paygate/pkg/database"
	"github.com/moov-io/paygate/pkg/transfers"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"
)

func between(amt string) error {
	if amt >= "USD 0.01" && amt <= "USD 0.25" {
		return nil
	}
	return fmt.Errorf("invalid amount %q", amt)
}

func TestAmountConditions(t *testing.T) {
	if err := between("USD 0.10"); err != nil {
		t.Error(err)
	}
	if err := between("USD 0.24"); err != nil {
		t.Error(err)
	}

	if err := between("USD 0.00"); err == nil {
		t.Error("expected error")
	}
	if err := between("USD 0.26"); err == nil {
		t.Error("expected error")
	}

	if err := between(""); err == nil {
		t.Error("expected error")
	}
	if err := between("invalid"); err == nil {
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
	userID := base.ID()

	db := database.CreateTestSqliteDB(t)
	t.Cleanup(func() { db.Close() })

	src, dest := createTestSource(cfg.ODFI), createTestDestination()

	repo := transfers.NewRepo(db.DB)
	decryptor := &accounts.MockDecryptor{
		Number: "12345",
	}
	pub := pipeline.NewMockPublisher()
	strategy := fundflow.NewFirstPerson(cfg.Logger, cfg.ODFI)

	micro, err := createMicroDeposits(*cfg.Validation.MicroDeposits, userID, src, dest, repo, decryptor, strategy, pub)
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
			case entries[0].RDFIIdentification == "98765432":
				if entries[0].TransactionCode != ach.CheckingCredit {
					t.Errorf("entries[0].TransactionCode=%d", entries[0].TransactionCode)
				}

			case entries[0].RDFIIdentification == "12345678":
				if entries[0].TransactionCode != ach.SavingsDebit {
					t.Errorf("entries[0].TransactionCode=%d", entries[0].TransactionCode)
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
			Status:     customers.VERIFIED,
		},
		Account: customers.Account{
			AccountID:     "src-account",
			RoutingNumber: odfi.RoutingNumber,
			Type:          customers.CHECKING,
		},
	}
}

func createTestDestination() fundflow.Destination {
	return fundflow.Destination{
		Customer: customers.Customer{
			CustomerID: "dest-customer",
			Status:     customers.VERIFIED,
		},
		Account: customers.Account{
			AccountID:     "dest-account",
			RoutingNumber: "987654320",
			Type:          customers.SAVINGS,
		},
		AccountNumber: "12345",
	}
}

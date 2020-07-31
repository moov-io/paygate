// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers/accounts"
	"github.com/moov-io/paygate/pkg/model"
	"github.com/moov-io/paygate/pkg/transfers"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"
)

func createMicroDeposits(
	cfg config.MicroDeposits,
	userID string,
	companyIdentification string,
	src fundflow.Source,
	dest fundflow.Destination,
	repo transfers.Repository,
	accountDecryptor accounts.Decryptor,
	strategy fundflow.Strategy,
	pub pipeline.XferPublisher,
) (*client.MicroDeposits, error) {

	amt1, amt2 := getMicroDepositAmounts()

	micro := &client.MicroDeposits{
		MicroDepositID: base.ID(),
		Destination: client.Destination{
			CustomerID: dest.Customer.CustomerID,
			AccountID:  dest.Account.AccountID,
		},
		Amounts: []string{amt1, amt2},
		Status:  client.PENDING,
		Created: time.Now(),
	}

	// originate two credits
	if xfer, err := originate(cfg, userID, companyIdentification, amt1, src, dest, repo, strategy, pub); err != nil {
		return nil, err
	} else {
		micro.TransferIDs = append(micro.TransferIDs, xfer.TransferID)
	}
	if xfer, err := originate(cfg, userID, companyIdentification, amt2, src, dest, repo, strategy, pub); err != nil {
		return nil, err
	} else {
		micro.TransferIDs = append(micro.TransferIDs, xfer.TransferID)
	}

	// originate the debit
	sum, err := model.SumAmounts(amt1, amt2)
	if err != nil {
		return micro, err
	}
	src, dest, err = flipSourceDest(src, dest, accountDecryptor)
	if err != nil {
		return micro, err
	}
	if xfer, err := originate(cfg, userID, companyIdentification, sum.String(), src, dest, repo, strategy, pub); err != nil {
		return micro, err
	} else {
		// Add the Transfer onto the MicroDeposit
		micro.TransferIDs = append(micro.TransferIDs, xfer.TransferID)
	}
	return micro, nil
}

func getMicroDepositAmounts() (string, string) {
	random := func() string {
		n, _ := rand.Int(rand.Reader, big.NewInt(25)) // rand.Int returns [0, N)
		return fmt.Sprintf("USD 0.%02d", int(n.Int64())+1)
	}
	return random(), random()
}

func originate(
	cfg config.MicroDeposits,
	userID string,
	companyIdentification string,
	amt string,
	source fundflow.Source,
	destination fundflow.Destination,
	transferRepo transfers.Repository,
	fundStrategy fundflow.Strategy,
	pub pipeline.XferPublisher,
) (*client.Transfer, error) {
	xfer := microDepositTransfer(amt, source, destination, cfg.Description, cfg.SameDay)

	// Save our Transfer to the database
	if err := transferRepo.WriteUserTransfer(userID, xfer); err != nil {
		return nil, err
	}

	// Originate ACH file(s) and send off to our Transfer publisher
	files, err := fundStrategy.Originate(companyIdentification, xfer, source, destination)
	if err != nil {
		return nil, err
	}
	if err := pipeline.PublishFiles(pub, xfer, files); err != nil {
		return nil, err
	}
	return xfer, nil
}

func flipSourceDest(src fundflow.Source, dest fundflow.Destination, accountDecryptor accounts.Decryptor) (fundflow.Source, fundflow.Destination, error) {
	number, err := accountDecryptor.AccountNumber(src.Customer.CustomerID, src.Account.AccountID)
	if err != nil {
		return fundflow.Source{}, fundflow.Destination{}, err
	}
	source := fundflow.Source(dest)
	destination := fundflow.Destination{
		Customer:      src.Customer,
		Account:       src.Account,
		AccountNumber: number,
	}
	return source, destination, nil
}

func microDepositTransfer(amt string, src fundflow.Source, dest fundflow.Destination, description string, sameDay bool) *client.Transfer {
	if description == "" {
		description = "validation"
	}
	return &client.Transfer{
		TransferID: base.ID(),
		Amount:     amt,
		Source: client.Source{
			CustomerID: src.Customer.CustomerID,
			AccountID:  src.Account.AccountID,
		},
		Destination: client.Destination{
			CustomerID: dest.Customer.CustomerID,
			AccountID:  dest.Account.AccountID,
		},
		Description: description,
		Status:      client.PENDING,
		SameDay:     sameDay,
		Created:     time.Now(),
	}
}

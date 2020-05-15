// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers"
	"github.com/moov-io/paygate/pkg/customers/accounts"
	"github.com/moov-io/paygate/pkg/tenants"
	"github.com/moov-io/paygate/pkg/transfers"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"
	"github.com/moov-io/paygate/x/route"

	"github.com/go-kit/kit/log"
)

type Router struct {
	GetUserTransfers   http.HandlerFunc
	CreateUserTransfer http.HandlerFunc
	GetUserTransfer    http.HandlerFunc
	DeleteUserTransfer http.HandlerFunc

	InitiateMicroDeposits   http.HandlerFunc
	GetMicroDeposits        http.HandlerFunc
	GetAccountMicroDeposits http.HandlerFunc
}

func NewRouter(
	cfg *config.Config,
	repo Repository,
	transferRepo transfers.Repository,
	tenantRepo tenants.Repository,
	customersClient customers.Client,
	accountDecryptor accounts.Decryptor,
	fundStrategy fundflow.Strategy,
	pub pipeline.XferPublisher,
) *Router {
	if cfg.Validation.MicroDeposits == nil {
		return &Router{
			InitiateMicroDeposits:   NotImplemented(cfg.Logger),
			GetMicroDeposits:        NotImplemented(cfg.Logger),
			GetAccountMicroDeposits: NotImplemented(cfg.Logger),
		}
	}
	config := *cfg.Validation.MicroDeposits
	return &Router{
		InitiateMicroDeposits:   InitiateMicroDeposits(config, cfg.Logger, repo, transferRepo, customersClient, accountDecryptor, fundStrategy, pub),
		GetMicroDeposits:        GetMicroDeposits(cfg.Logger, repo),
		GetAccountMicroDeposits: GetAccountMicroDeposits(cfg.Logger, repo),
	}
}

func (c *Router) RegisterRoutes(r *mux.Router) {
	r.Methods("POST").Path("/micro-deposits").HandlerFunc(c.InitiateMicroDeposits)
	r.Methods("GET").Path("/micro-deposits/{microDepositID}").HandlerFunc(c.GetMicroDeposits)
	r.Methods("GET").Path("/accounts/{accountID}/micro-deposits").HandlerFunc(c.GetAccountMicroDeposits)
}

func InitiateMicroDeposits(
	cfg config.MicroDeposits,
	logger log.Logger,
	repo Repository,
	transferRepo transfers.Repository,
	customersClient customers.Client,
	accountDecryptor accounts.Decryptor,
	fundStrategy fundflow.Strategy,
	pub pipeline.XferPublisher,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		var req client.CreateMicroDeposits
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			responder.Problem(err)
			return
		}

		src, err := getMicroDepositSource(cfg, customersClient)
		if err != nil {
			responder.Problem(err)
			return
		}
		dest, err := transfers.GetFundflowDestination(customersClient, accountDecryptor, req.Destination)
		if err != nil {
			responder.Problem(err)
			return
		}

		// TODO(adam): we need to schedule the debit. can't we initiate the debit right away though?

		amt1, amt2 := getMicroDepositAmounts()
		var transfers []*client.Transfer

		if xfer, err := sendOffMicroDeposit(cfg, responder.XUserID, amt1, src, dest, transferRepo, fundStrategy, pub); err != nil {
			responder.Problem(err)
			return
		} else {
			transfers = append(transfers, xfer)
		}
		if xfer, err := sendOffMicroDeposit(cfg, responder.XUserID, amt2, src, dest, transferRepo, fundStrategy, pub); err != nil {
			responder.Problem(err)
			return
		} else {
			transfers = append(transfers, xfer)
		}

		micro := client.MicroDeposits{
			MicroDepositID: base.ID(),
			TransferIDs: []string{
				transfers[0].TransferID, transfers[1].TransferID,
			},
			Destination: client.Destination{
				CustomerID: dest.Customer.CustomerID,
				AccountID:  dest.Account.AccountID,
			},
			Amounts: []string{
				amt1, amt2,
			},
			Status:  client.PENDING,
			Created: time.Now(),
		}

		if err := repo.writeMicroDeposits(micro); err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(micro)
		})
	}
}

func getMicroDepositAmounts() (string, string) {
	random := func() string {
		n, _ := rand.Int(rand.Reader, big.NewInt(25)) // rand.Int returns [0, N)
		return fmt.Sprintf("USD 0.%02d", int(n.Int64())+1)
	}
	return random(), random()
}

func getMicroDepositSource(cfg config.MicroDeposits, customersClient customers.Client) (fundflow.Source, error) {
	return transfers.GetFundflowSource(customersClient, client.Source{
		CustomerID: cfg.Source.CustomerID,
		AccountID:  cfg.Source.AccountID,
	})
}

func createMicroDepositTransfer(amt string, src fundflow.Source, dest fundflow.Destination) *client.Transfer {
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
		Description: "account validation",
		Status:      client.PENDING,
		SameDay:     false,
		Created:     time.Now(),
	}
}

func sendOffMicroDeposit(
	cfg config.MicroDeposits,
	userID string,
	amt string,
	source fundflow.Source,
	destination fundflow.Destination,
	transferRepo transfers.Repository,
	fundStrategy fundflow.Strategy,
	pub pipeline.XferPublisher,
) (*client.Transfer, error) {
	xfer := createMicroDepositTransfer(amt, source, destination)

	// Save our Transfer to the database
	if err := transferRepo.WriteUserTransfer(userID, xfer); err != nil {
		return nil, err
	}

	// Originate ACH file(s) and send off to our Transfer publisher
	files, err := fundStrategy.Originate(config.CompanyID, xfer, source, destination)
	if err != nil {
		return nil, err
	}
	if err := pipeline.PublishFiles(pub, xfer, files); err != nil {
		return nil, err
	}
	return xfer, nil
}

func GetMicroDeposits(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		microDepositID := route.ReadPathID("microDepositID", r)
		micro, err := repo.getMicroDeposits(microDepositID)
		if err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(micro)
		})
	}
}

func GetAccountMicroDeposits(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)

		accountID := route.ReadPathID("accountID", r)
		micro, err := repo.getAccountMicroDeposits(accountID)
		if err != nil {
			responder.Problem(err)
			return
		}

		responder.Respond(func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(micro)
		})
	}
}

func NotImplemented(logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(logger, w, r)
		responder.Problem(errors.New("micro-deposits are disabled via config"))
	}
}

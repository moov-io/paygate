// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposits

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/moov-io/base/log"
	moovcustomers "github.com/moov-io/customers/pkg/client"

	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/customers"
	"github.com/moov-io/paygate/pkg/customers/accounts"
	"github.com/moov-io/paygate/pkg/transfers"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"
	"github.com/moov-io/paygate/x/route"
)

type Router struct {
	InitiateMicroDeposits   http.HandlerFunc
	GetMicroDeposits        http.HandlerFunc
	GetAccountMicroDeposits http.HandlerFunc
}

func NewRouter(
	cfg *config.Config,
	repo Repository,
	transferRepo transfers.Repository,
	customersClient customers.Client,
	accountDecryptor accounts.Decryptor,
	fundStrategy fundflow.Strategy,
	pub pipeline.XferPublisher,
) *Router {
	if cfg.Validation.MicroDeposits == nil {
		return &Router{
			InitiateMicroDeposits:   NotImplemented(cfg),
			GetMicroDeposits:        NotImplemented(cfg),
			GetAccountMicroDeposits: NotImplemented(cfg),
		}
	}

	// companyIdentification is the similarly named Batch Header field. It can be
	// overridden from auth on the request.
	// TODO(adam): this will also be read from auth on the request
	companyIdentification := cfg.ODFI.FileConfig.BatchHeader.CompanyIdentification

	return &Router{
		InitiateMicroDeposits:   InitiateMicroDeposits(cfg, companyIdentification, repo, transferRepo, customersClient, accountDecryptor, fundStrategy, pub),
		GetMicroDeposits:        GetMicroDeposits(cfg, repo),
		GetAccountMicroDeposits: GetAccountMicroDeposits(cfg, repo),
	}
}

func (c *Router) RegisterRoutes(r *mux.Router) {
	r.Methods("POST").Path("/micro-deposits").HandlerFunc(c.InitiateMicroDeposits)
	r.Methods("GET").Path("/micro-deposits/{microDepositID}").HandlerFunc(c.GetMicroDeposits)
	r.Methods("GET").Path("/accounts/{accountID}/micro-deposits").HandlerFunc(c.GetAccountMicroDeposits)
}

func InitiateMicroDeposits(
	cfg *config.Config,
	companyIdentification string,
	repo Repository,
	transferRepo transfers.Repository,
	customersClient customers.Client,
	accountDecryptor accounts.Decryptor,
	fundStrategy fundflow.Strategy,
	pub pipeline.XferPublisher,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conf := *cfg.Validation.MicroDeposits
		cfg.Logger = cfg.Logger.Set("service", log.String("micro-deposits"))

		responder := route.NewResponder(cfg, w, r)
		responder.Respond(func(w http.ResponseWriter) {
			var req client.CreateMicroDeposits
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				responder.Problem(err)
				return
			}

			src, err := getMicroDepositSource(conf, customersClient, accountDecryptor)
			if err != nil {
				cfg.Logger.LogErrorf("ERROR getting micro-deposit source: %v", err)
				responder.Problem(err)
				return
			}
			dest, err := transfers.GetFundflowDestination(customersClient, accountDecryptor, req.Destination, responder.OrganizationID)
			if err != nil {
				cfg.Logger.LogErrorf("ERROR getting micro-deposit destination: %v", err)
				responder.Problem(err)
				return
			}
			if src.Account.RoutingNumber == dest.Account.RoutingNumber {
				err = errors.New("not initiating micro-deposits for account at ODFI")
				cfg.Logger.LogError(err)
				responder.Problem(err)
				return
			}
			if err := acceptableAccountStatus(dest.Account); err != nil {
				cfg.Logger.LogErrorf("destination account: %v", err)
				responder.Problem(err)
				return
			}

			micro, err := createMicroDeposits(conf, responder.OrganizationID, companyIdentification, src, dest, transferRepo, accountDecryptor, fundStrategy, pub)
			if err != nil {
				cfg.Logger.LogErrorf("ERROR creating micro-deposits: %v", err)
				responder.Problem(err)
				return
			}
			if err := repo.writeMicroDeposits(micro); err != nil {
				cfg.Logger.LogErrorf("ERROR writing micro-deposits: %v", err)
				responder.Problem(err)
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(micro)
		})
	}
}

func getMicroDepositSource(cfg config.MicroDeposits, customersClient customers.Client, accountDecryptor accounts.Decryptor) (fundflow.Source, error) {
	return transfers.GetFundflowSource(customersClient, accountDecryptor, client.Source{
		CustomerID: cfg.Source.CustomerID,
		AccountID:  cfg.Source.AccountID,
	}, cfg.Source.Organization)
}

func acceptableAccountStatus(acct moovcustomers.Account) error {
	if strings.EqualFold(string(acct.Status), string(moovcustomers.ACCOUNTSTATUS_NONE)) {
		return nil
	}
	return fmt.Errorf("accountID=%s is un unacceptable status: %v", acct.AccountID, acct.Status)
}

func GetMicroDeposits(cfg *config.Config, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(cfg, w, r)
		responder.Respond(func(w http.ResponseWriter) {
			microDepositID := route.ReadPathID("microDepositID", r)
			if microDepositID == "" {
				responder.Problem(errors.New("missing microDepositID"))
				return
			}

			micro, err := repo.getMicroDeposits(microDepositID)
			if err != nil && err != sql.ErrNoRows {
				cfg.Logger.LogErrorf("ERROR getting micro-deposits: %v", err)
				responder.Problem(err)
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(micro)
		})
	}
}

func GetAccountMicroDeposits(cfg *config.Config, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(cfg, w, r)
		responder.Respond(func(w http.ResponseWriter) {
			accountID := route.ReadPathID("accountID", r)
			if accountID == "" {
				responder.Problem(errors.New("missing accountID"))
				return
			}

			micro, err := repo.getAccountMicroDeposits(accountID)
			if err != nil && err != sql.ErrNoRows {
				cfg.Logger.LogErrorf("ERROR getting accountID=%s micro-deposits: %v", accountID, err)
				responder.Problem(err)
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(micro)
		})
	}
}

func NotImplemented(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responder := route.NewResponder(cfg, w, r)
		responder.Problem(errors.New("micro-deposits are disabled via config"))
	}
}

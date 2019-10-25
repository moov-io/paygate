// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/paygate"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/filetransfer"
	"github.com/moov-io/paygate/internal/microdeposit"
	"github.com/moov-io/paygate/internal/ofac"
	"github.com/moov-io/paygate/internal/util"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/gorilla/mux"
)

var (
	httpAddr  = flag.String("http.addr", bind.HTTP("paygate"), "HTTP listen address")
	adminAddr = flag.String("admin.addr", bind.Admin("paygate"), "Admin HTTP listen address")

	flagConfigFile = flag.String("config", "", "Filepath for config file to load")
	flagLogFormat  = flag.String("log.format", "", "Format for log lines (Options: json, plain")
)

func main() {
	configFilepath := util.Or(os.Getenv("CONFIG_FILEPATH"), *flagConfigFile)
	cfg, err := config.LoadConfig(configFilepath)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	flag.Parse()

	cfg.Logger.Log("startup", fmt.Sprintf("Starting paygate server version %s", paygate.Version))

	// migrate database
	db, err := database.New(cfg.Logger, cfg)
	if err != nil {
		panic(fmt.Sprintf("error creating database: %v", err))
	}
	defer func() {
		if err := db.Close(); err != nil {
			cfg.Logger.Log("exit", err)
		}
	}()

	// Listen for application termination.
	errs := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-c)
	}()

	// Spin up admin HTTP server and optionally override -admin.addr
	if *adminAddr != "" {
		cfg.Web.AdminBindAddress = *adminAddr
	}
	adminServer := admin.NewServer(cfg.Web.AdminBindAddress)
	adminServer.AddVersionHandler(paygate.Version) // Setup 'GET /version'
	go func() {
		cfg.Logger.Log("admin", fmt.Sprintf("listening on %s", adminServer.BindAddr()))
		if err := adminServer.Listen(); err != nil {
			err = fmt.Errorf("problem starting admin http: %v", err)
			cfg.Logger.Log("admin", err)
			errs <- err
		}
	}()
	defer adminServer.Shutdown()

	// Setup repositories
	receiverRepo := internal.NewReceiverRepo(cfg.Logger, db)
	defer receiverRepo.Close()

	depositoryRepo := internal.NewDepositoryRepo(cfg.Logger, db)
	defer depositoryRepo.Close()

	eventRepo := internal.NewEventRepo(cfg.Logger, db)
	defer eventRepo.Close()

	gatewaysRepo := internal.NewGatewayRepo(cfg.Logger, db)
	defer gatewaysRepo.Close()

	originatorsRepo := internal.NewOriginatorRepo(cfg.Logger, db)
	defer originatorsRepo.Close()

	transferRepo := internal.NewTransferRepo(cfg.Logger, db)
	defer transferRepo.Close()

	httpClient, err := internal.TLSHttpClient(cfg.Web.ClientCAFile)
	if err != nil {
		panic(fmt.Sprintf("problem creating TLS ready *http.Client: %v", err))
	}

	// Create our various Client instances
	achClient := setupACHClient(cfg, adminServer, httpClient)
	fedClient := setupFEDClient(cfg, adminServer, httpClient)
	ofacClient := setupOFACClient(cfg, adminServer, httpClient)

	// Bring up our Accounts Client
	accountsClient := setupAccountsClient(cfg, adminServer, httpClient)

	// Start our periodic file operations
	fileTransferRepo := filetransfer.NewRepository(cfg.Logger, configFilepath, db, cfg.DatabaseType)
	defer fileTransferRepo.Close()
	if err := filetransfer.ValidateTemplates(fileTransferRepo); err != nil {
		panic(fmt.Sprintf("ERROR: problem validating outbound filename templates: %v", err))
	}

	setupACHStorageDir(cfg)
	fileTransferController, err := filetransfer.NewController(cfg, fileTransferRepo, achClient, accountsClient)
	if err != nil {
		panic(fmt.Sprintf("ERROR: creating ACH file transfer controller: %v", err))
	}
	shutdownFileTransferController := setupFileTransferController(cfg, fileTransferController, depositoryRepo, fileTransferRepo, transferRepo, adminServer)
	defer shutdownFileTransferController()

	// Register the micro-deposit admin route
	microdeposit.RegisterAdminRoutes(cfg.Logger, adminServer, depositoryRepo)

	// Create HTTP handler
	handler := mux.NewRouter()
	internal.AddReceiverRoutes(cfg.Logger, handler, ofacClient, receiverRepo, depositoryRepo)
	internal.AddEventRoutes(cfg.Logger, handler, eventRepo)
	internal.AddGatewayRoutes(cfg.Logger, handler, gatewaysRepo)
	internal.AddOriginatorRoutes(cfg.Logger, handler, accountsClient, ofacClient, depositoryRepo, originatorsRepo)
	internal.AddPingRoute(cfg.Logger, handler)

	// Depository HTTP routes
	odfiAccount := setupODFIAccount(cfg, accountsClient)
	depositoryRouter := internal.NewDepositoryRouter(cfg, odfiAccount, accountsClient, achClient, fedClient, ofacClient, depositoryRepo, eventRepo)
	depositoryRouter.RegisterRoutes(handler)

	// Transfer HTTP routes
	achClientFactory := func(userId string) *achclient.ACH {
		return achclient.New(cfg.Logger, cfg.ACH.Endpoint, userId, httpClient)
	}
	xferRouter := internal.NewTransferRouter(cfg.Logger, depositoryRepo, eventRepo, receiverRepo, originatorsRepo, transferRepo, achClientFactory, accountsClient)
	xferRouter.RegisterRoutes(handler)

	// Check to see if our -http.addr flag has been overridden
	if *httpAddr != "" {
		cfg.Web.BindAddress = *httpAddr
	}
	// Create main HTTP server
	serve := &http.Server{
		Addr:    cfg.Web.BindAddress,
		Handler: handler,
		TLSConfig: &tls.Config{
			InsecureSkipVerify:       false,
			PreferServerCipherSuites: true,
			MinVersion:               tls.VersionTLS12,
		},
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	shutdownServer := func() {
		if err := serve.Shutdown(context.TODO()); err != nil {
			cfg.Logger.Log("shutdown", err)
		}
	}
	defer shutdownServer()

	// Start main HTTP server
	go func() {
		if cfg.Web.CertFile != "" && cfg.Web.KeyFile != "" {
			cfg.Logger.Log("startup", fmt.Sprintf("binding to %s for secure HTTP server", *httpAddr))
			if err := serve.ListenAndServeTLS(cfg.Web.CertFile, cfg.Web.KeyFile); err != nil {
				cfg.Logger.Log("exit", err)
			}
		} else {
			cfg.Logger.Log("startup", fmt.Sprintf("binding to %s for HTTP server", *httpAddr))
			if err := serve.ListenAndServe(); err != nil {
				cfg.Logger.Log("exit", err)
			}
		}
	}()

	if err := <-errs; err != nil {
		cfg.Logger.Log("exit", err)
	}
}

func setupACHClient(cfg *config.Config, svc *admin.Server, httpClient *http.Client) *achclient.ACH {
	client := achclient.New(cfg.Logger, cfg.ACH.Endpoint, "ach", httpClient)
	if client == nil {
		panic("no ACH client created")
	}
	svc.AddLivenessCheck("ach", client.Ping)
	return client
}

func setupACHStorageDir(cfg *config.Config) {
	if cfg.ACH.StorageDir == "" {
		cfg.ACH.StorageDir = "./storage/"
	}
	if dir := filepath.Dir(cfg.ACH.StorageDir); dir != "" {
		if err := os.MkdirAll(dir, 0777); err != nil {
			panic(fmt.Sprintf("problem creating %s: %v", dir, err))
		}
		cfg.Logger.Log("storage", fmt.Sprintf("using %s for storage directory", dir))
	}
}

func setupAccountsClient(cfg *config.Config, svc *admin.Server, httpClient *http.Client) internal.AccountsClient {
	if cfg.Accounts.Disabled {
		return nil
	}
	accountsClient := internal.CreateAccountsClient(cfg.Logger, cfg.Accounts.Endpoint, httpClient)
	if accountsClient == nil {
		panic("no Accounts client created")
	}
	svc.AddLivenessCheck("accounts", accountsClient.Ping)
	return accountsClient
}

func setupFEDClient(cfg *config.Config, svc *admin.Server, httpClient *http.Client) fed.Client {
	client := fed.NewClient(cfg.Logger, cfg.FED.Endpoint, httpClient)
	if client == nil {
		panic("no FED client created")
	}
	svc.AddLivenessCheck("fed", client.Ping)
	return client
}

func setupODFIAccount(cfg *config.Config, accountsClient internal.AccountsClient) *internal.ODFIAccount {
	odfiAccountType := internal.Savings
	if cfg.ODFI.AccountType != "" {
		t := internal.AccountType(cfg.ODFI.AccountType)
		if err := t.Validate(); err == nil {
			odfiAccountType = t
		}
	}

	accountNumber := util.Or(cfg.ODFI.AccountNumber, "123")
	routingNumber := util.Or(cfg.ODFI.RoutingNumber, "121042882")

	return internal.NewODFIAccount(accountsClient, accountNumber, routingNumber, odfiAccountType)
}

func setupOFACClient(cfg *config.Config, svc *admin.Server, httpClient *http.Client) ofac.Client {
	client := ofac.NewClient(cfg.Logger, cfg.OFAC.Endpoint, httpClient)
	if client == nil {
		panic("no OFAC client created")
	}
	svc.AddLivenessCheck("ofac", client.Ping)
	return client
}

func setupFileTransferController(
	cfg *config.Config,
	controller *filetransfer.Controller,
	depRepo internal.DepositoryRepository,
	fileTransferRepo filetransfer.Repository,
	transferRepo internal.TransferRepository,
	svc *admin.Server,
) context.CancelFunc {
	ctx, cancelFileSync := context.WithCancel(context.Background())

	if controller == nil {
		return cancelFileSync
	}

	flushIncoming, flushOutgoing := make(filetransfer.FlushChan, 1), make(filetransfer.FlushChan, 1) // buffered channels to allow only one concurrent operation

	// start our controller's operations in an anon goroutine
	go controller.StartPeriodicFileOperations(ctx, flushIncoming, flushOutgoing, depRepo, transferRepo)

	filetransfer.AddFileTransferConfigRoutes(cfg.Logger, svc, fileTransferRepo)
	filetransfer.AddFileTransferSyncRoute(cfg.Logger, svc, flushIncoming, flushOutgoing)

	return cancelFileSync
}

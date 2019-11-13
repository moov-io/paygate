// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/tls"
	"database/sql"
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
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/features"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/filetransfer"
	"github.com/moov-io/paygate/internal/microdeposit"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/internal/util"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

var (
	httpAddr  = flag.String("http.addr", bind.HTTP("paygate"), "HTTP listen address")
	adminAddr = flag.String("admin.addr", bind.Admin("paygate"), "Admin HTTP listen address")

	flagConfigFile = flag.String("config", "", "Filepath for config file to load")
	flagLogFormat  = flag.String("log.format", "", "Format for log lines (Options: json, plain")
)

func main() {
	flag.Parse()

	configFilepath := util.Or(os.Getenv("CONFIG_FILE"), *flagConfigFile)
	cfg, err := config.LoadConfig(configFilepath, flagLogFormat)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	cfg.Logger.Log("startup", fmt.Sprintf("Starting paygate server version %s", paygate.Version))

	// migrate database
	db, err := database.New(cfg.Logger, os.Getenv("DATABASE_TYPE"))
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
	if v := os.Getenv("HTTP_ADMIN_BIND_ADDRESS"); v != "" {
		*adminAddr = v
	}
	adminServer := admin.NewServer(*adminAddr)
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

	httpClient, err := internal.TLSHttpClient(os.Getenv("HTTP_CLIENT_CAFILE"))
	if err != nil {
		panic(fmt.Sprintf("problem creating TLS ready *http.Client: %v", err))
	}

	// Create our various Client instances
	achClient := setupACHClient(cfg.Logger, os.Getenv("ACH_ENDPOINT"), adminServer, httpClient)
	fedClient := setupFEDClient(cfg.Logger, os.Getenv("FED_ENDPOINT"), adminServer, httpClient)

	// Bring up our Accounts Client
	accountsClient := setupAccountsClient(cfg.Logger, adminServer, httpClient, os.Getenv("ACCOUNTS_ENDPOINT"), os.Getenv("ACCOUNTS_CALLS_DISABLED"))
	accountsCallsDisabled := accountsClient == nil

	customersClient := setupCustomersClient(cfg, adminServer, httpClient)
	customersCallsDisabled := customersClient == nil

	customerOFACRefresher := setupCustomersRefresher(cfg, customersClient, db)
	if customerOFACRefresher != nil {
		defer customerOFACRefresher.Close()
	}

	features.AddRoutes(cfg.Logger, adminServer, accountsCallsDisabled, customersCallsDisabled)

	// Start our periodic file operations
	fileTransferRepo := filetransfer.NewRepository(configFilepath, db, os.Getenv("DATABASE_TYPE"))
	defer fileTransferRepo.Close()
	if err := filetransfer.ValidateTemplates(fileTransferRepo); err != nil {
		panic(fmt.Sprintf("ERROR: problem validating outbound filename templates: %v", err))
	}

	achStorageDir := setupACHStorageDir(cfg.Logger)
	fileTransferController, err := filetransfer.NewController(cfg, achStorageDir, fileTransferRepo, achClient, accountsClient)
	if err != nil {
		panic(fmt.Sprintf("ERROR: creating ACH file transfer controller: %v", err))
	}
	shutdownFileTransferController := setupFileTransferController(cfg.Logger, fileTransferController, depositoryRepo, fileTransferRepo, transferRepo, adminServer)
	defer shutdownFileTransferController()

	// Register the micro-deposit admin route
	microdeposit.RegisterAdminRoutes(cfg.Logger, adminServer, depositoryRepo)

	// Create HTTP handler
	handler := mux.NewRouter()
	internal.AddReceiverRoutes(cfg.Logger, handler, customersClient, depositoryRepo, receiverRepo)
	internal.AddEventRoutes(cfg.Logger, handler, eventRepo)
	internal.AddGatewayRoutes(cfg.Logger, handler, gatewaysRepo)
	internal.AddOriginatorRoutes(cfg.Logger, handler, accountsClient, customersClient, depositoryRepo, originatorsRepo)
	internal.AddPingRoute(cfg.Logger, handler)

	keeper, err := secrets.OpenSecretKeeper(context.Background(), "", "CLOUD_PROVIDER")
	if err != nil {
		panic(err)
	}
	stringKeeper := secrets.NewStringKeeper(keeper, 10*time.Second)

	// Depository HTTP routes
	odfiAccount := setupODFIAccount(accountsClient)
	depositoryRouter := internal.NewDepositoryRouter(cfg.Logger, odfiAccount, accountsClient, achClient, fedClient, depositoryRepo, eventRepo, stringKeeper)
	depositoryRouter.RegisterRoutes(handler)

	// Transfer HTTP routes
	achClientFactory := func(userId string) *achclient.ACH {
		return achclient.New(cfg.Logger, os.Getenv("ACH_ENDPOINT"), userId, httpClient)
	}
	xferRouter := internal.NewTransferRouter(cfg.Logger, depositoryRepo, eventRepo, receiverRepo, originatorsRepo, transferRepo, achClientFactory, accountsClient, customersClient)
	xferRouter.RegisterRoutes(handler)

	// Check to see if our -http.addr flag has been overridden
	if v := os.Getenv("HTTP_BIND_ADDRESS"); v != "" {
		*httpAddr = v
	}
	// Create main HTTP server
	serve := &http.Server{
		Addr:    *httpAddr,
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
		if certFile, keyFile := os.Getenv("HTTPS_CERT_FILE"), os.Getenv("HTTPS_KEY_FILE"); certFile != "" && keyFile != "" {
			cfg.Logger.Log("startup", fmt.Sprintf("binding to %s for secure HTTP server", *httpAddr))
			if err := serve.ListenAndServeTLS(certFile, keyFile); err != nil {
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

func setupACHClient(logger log.Logger, endpoint string, svc *admin.Server, httpClient *http.Client) *achclient.ACH {
	client := achclient.New(logger, endpoint, "ach", httpClient)
	if client == nil {
		panic("no ACH client created")
	}
	svc.AddLivenessCheck("ach", client.Ping)
	return client
}

func setupAccountsClient(logger log.Logger, svc *admin.Server, httpClient *http.Client, endpoint, disabled string) internal.AccountsClient {
	if util.Yes(disabled) {
		return nil
	}
	accountsClient := internal.CreateAccountsClient(logger, endpoint, httpClient)
	if accountsClient == nil {
		panic("no Accounts client created")
	}
	svc.AddLivenessCheck("accounts", accountsClient.Ping)
	return accountsClient
}

func setupCustomersClient(cfg *config.Config, svc *admin.Server, httpClient *http.Client) customers.Client {
	if cfg.Customers.Disabled {
		return nil
	}
	client := customers.NewClient(cfg.Logger, cfg.Customers.Endpoint, httpClient)
	if client == nil {
		panic("no Customers client created")
	}
	svc.AddLivenessCheck("customers", client.Ping)
	return client
}

func setupCustomersRefresher(cfg *config.Config, client customers.Client, db *sql.DB) internal.Refresher {
	refresher := internal.NewRefresher(cfg, client, db)
	if refresher != nil {
		go func() {
			if err := refresher.Start(cfg.Customers.OFACRefreshEvery); err != nil {
				cfg.Logger.Log("customers", fmt.Errorf("problem with OFAC refresher: %v", err))
			}
		}()
	}
	return refresher
}

func setupFEDClient(logger log.Logger, endpoint string, svc *admin.Server, httpClient *http.Client) fed.Client {
	client := fed.NewClient(logger, endpoint, httpClient)
	if client == nil {
		panic("no FED client created")
	}
	svc.AddLivenessCheck("fed", client.Ping)
	return client
}

func setupODFIAccount(accountsClient internal.AccountsClient) *internal.ODFIAccount {
	odfiAccountType := internal.Savings
	if v := os.Getenv("ODFI_ACCOUNT_TYPE"); v != "" {
		t := internal.AccountType(v)
		if err := t.Validate(); err == nil {
			odfiAccountType = t
		}
	}

	accountNumber := util.Or(os.Getenv("ODFI_ACCOUNT_NUMBER"), "123")
	routingNumber := util.Or(os.Getenv("ODFI_ROUTING_NUMBER"), "121042882")

	return internal.NewODFIAccount(accountsClient, accountNumber, routingNumber, odfiAccountType)
}

func setupACHStorageDir(logger log.Logger) string {
	dir := filepath.Dir(os.Getenv("ACH_FILE_STORAGE_DIR"))
	if dir == "." {
		dir = "./storage/"
	}
	if err := os.MkdirAll(dir, 0777); err != nil {
		panic(fmt.Sprintf("problem creating %s: %v", dir, err))
	}
	logger.Log("storage", fmt.Sprintf("using %s for storage directory", dir))
	return dir
}

func setupFileTransferController(logger log.Logger, controller *filetransfer.Controller, depRepo internal.DepositoryRepository, fileTransferRepo filetransfer.Repository, transferRepo internal.TransferRepository, svc *admin.Server) context.CancelFunc {
	ctx, cancelFileSync := context.WithCancel(context.Background())

	if controller == nil {
		return cancelFileSync
	}

	flushIncoming, flushOutgoing := make(filetransfer.FlushChan, 1), make(filetransfer.FlushChan, 1) // buffered channels to allow only one concurrent operation

	// start our controller's operations in an anon goroutine
	go controller.StartPeriodicFileOperations(ctx, flushIncoming, flushOutgoing, depRepo, transferRepo)

	filetransfer.AddFileTransferConfigRoutes(logger, svc, fileTransferRepo)
	filetransfer.AddFileTransferSyncRoute(logger, svc, flushIncoming, flushOutgoing)

	return cancelFileSync
}

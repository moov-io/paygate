// Copyright 2020 The Moov Authors
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
	"github.com/moov-io/paygate/internal/accounts"
	appcfg "github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/customers"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/depository/verification/microdeposit"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/features"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/filetransfer"
	ftadmin "github.com/moov-io/paygate/internal/filetransfer/admin"
	"github.com/moov-io/paygate/internal/filetransfer/config"
	"github.com/moov-io/paygate/internal/gateways"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/ofac"
	"github.com/moov-io/paygate/internal/originators"
	"github.com/moov-io/paygate/internal/receivers"
	"github.com/moov-io/paygate/internal/route"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/internal/transfers"
	"github.com/moov-io/paygate/internal/util"

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
	cfg, err := appcfg.LoadConfig(configFilepath, flagLogFormat)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	cfg.Logger.Log("startup", fmt.Sprintf("Starting paygate server version %s", paygate.Version))

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	// migrate database
	db, err := database.New(ctx, cfg.Logger, database.Type())
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

	keeper, err := secrets.OpenSecretKeeper(ctx, "paygate-account-numbers", os.Getenv("CLOUD_PROVIDER"))
	if err != nil {
		panic(err)
	}
	stringKeeper := secrets.NewStringKeeper(keeper, 10*time.Second)

	// Setup repositories
	receiverRepo := receivers.NewReceiverRepo(cfg.Logger, db)
	defer receiverRepo.Close()

	depositoryRepo := depository.NewDepositoryRepo(cfg.Logger, db, stringKeeper)
	defer depositoryRepo.Close()

	// if err := internal.EncryptStoredAccountNumbers(cfg.Logger, depositoryRepo, stringKeeper); err != nil {
	// 	panic(err)
	// }

	eventRepo := events.NewRepo(cfg.Logger, db)
	defer eventRepo.Close()

	gatewaysRepo := gateways.NewRepo(cfg.Logger, db)
	defer gatewaysRepo.Close()

	originatorsRepo := originators.NewOriginatorRepo(cfg.Logger, db)
	defer originatorsRepo.Close()

	transferRepo := transfers.NewTransferRepo(cfg.Logger, db)
	defer transferRepo.Close()

	httpClient, err := route.TLSHttpClient(os.Getenv("HTTP_CLIENT_CAFILE"))
	if err != nil {
		panic(fmt.Sprintf("problem creating TLS ready *http.Client: %v", err))
	}

	// Create our various Client instances
	fedClient := setupFEDClient(cfg.Logger, os.Getenv("FED_ENDPOINT"), os.Getenv("FED_CALLS_DISABLED"), adminServer, httpClient)

	// Bring up our Accounts Client
	accountsClient := setupAccountsClient(cfg.Logger, adminServer, httpClient, os.Getenv("ACCOUNTS_ENDPOINT"), os.Getenv("ACCOUNTS_CALLS_DISABLED"))
	accountsCallsDisabled := accountsClient == nil

	customersClient := setupCustomersClient(cfg, adminServer, httpClient)
	customersCallsDisabled := customersClient == nil

	customerOFACRefresher := setupOFACRefresher(cfg, customersClient, db)
	if customerOFACRefresher != nil {
		defer customerOFACRefresher.Close()
	}

	features.AddRoutes(cfg.Logger, adminServer, accountsCallsDisabled, customersCallsDisabled)

	// Start our periodic file operations
	fileTransferRepo := config.NewRepository(configFilepath, db, os.Getenv("DATABASE_TYPE"))
	defer fileTransferRepo.Close()
	if err := config.ValidateTemplates(fileTransferRepo); err != nil {
		panic(fmt.Sprintf("ERROR: problem validating outbound filename templates: %v", err))
	}

	// micro-deposit repository
	microDepositRepo := microdeposit.NewRepository(cfg.Logger, db)

	achStorageDir := setupACHStorageDir(cfg.Logger)
	fileTransferController, err := filetransfer.NewController(cfg, achStorageDir, fileTransferRepo, depositoryRepo, microDepositRepo, transferRepo, accountsClient)
	if err != nil {
		panic(fmt.Sprintf("ERROR: creating ACH file transfer controller: %v", err))
	}
	shutdownFileTransferController, removalChan := setupFileTransferController(cfg.Logger, fileTransferController, depositoryRepo, fileTransferRepo, microDepositRepo, transferRepo, adminServer)
	defer shutdownFileTransferController()

	// Register the micro-deposit admin route
	depository.RegisterAdminRoutes(cfg.Logger, adminServer, depositoryRepo)

	// Create HTTP handler
	handler := mux.NewRouter()
	receivers.AddReceiverRoutes(cfg.Logger, handler, customersClient, depositoryRepo, receiverRepo)
	events.AddRoutes(cfg.Logger, handler, eventRepo)
	gateways.AddRoutes(cfg.Logger, handler, gatewaysRepo)
	originators.AddOriginatorRoutes(cfg.Logger, handler, accountsClient, customersClient, depositoryRepo, originatorsRepo)
	route.AddPingRoute(cfg.Logger, handler)

	// Depository HTTP routes
	depositoryRouter := depository.NewRouter(cfg.Logger, fedClient, depositoryRepo, eventRepo, stringKeeper, removalChan)
	depositoryRouter.RegisterRoutes(handler)

	// Gateway HTTP Routes
	gatewayRepo := gateways.NewRepo(cfg.Logger, db)
	gateways.AddRoutes(cfg.Logger, handler, gatewayRepo)

	// MicroDeposit HTTP routes
	attempter := microdeposit.NewAttemper(cfg.Logger, db, 5)
	odfiAccount := setupODFIAccount(accountsClient, stringKeeper)
	microDepositRouter := microdeposit.NewRouter(cfg.Logger, odfiAccount, attempter, accountsClient, depositoryRepo, eventRepo, gatewayRepo, microDepositRepo)
	microDepositRouter.RegisterRoutes(handler)

	// Transfer HTTP routes
	limits, err := transfers.ParseLimits(transfers.OneDayLimit(), transfers.SevenDayLimit(), transfers.ThirtyDayLimit())
	if err != nil {
		panic(fmt.Sprintf("ERROR parsing transfer limits: %v", err))
	}
	transferLimitChecker := transfers.NewLimitChecker(cfg.Logger, db, limits)
	xferRouter := transfers.NewTransferRouter(cfg.Logger,
		depositoryRepo, eventRepo, gatewayRepo, receiverRepo, originatorsRepo, transferRepo,
		transferLimitChecker, removalChan,
		accountsClient, customersClient,
	)
	xferRouter.RegisterRoutes(handler)
	transfers.RegisterAdminRoutes(cfg.Logger, adminServer, transferRepo)

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

func setupAccountsClient(logger log.Logger, svc *admin.Server, httpClient *http.Client, endpoint, disabled string) accounts.Client {
	if util.Yes(disabled) {
		return nil
	}
	accountsClient := accounts.NewClient(logger, endpoint, httpClient)
	if accountsClient == nil {
		panic("no Accounts client created")
	}
	svc.AddLivenessCheck("accounts", accountsClient.Ping)
	return accountsClient
}

func setupCustomersClient(cfg *appcfg.Config, svc *admin.Server, httpClient *http.Client) customers.Client {
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

func setupOFACRefresher(cfg *appcfg.Config, client customers.Client, db *sql.DB) ofac.Refresher {
	refresher := ofac.NewRefresher(cfg, client, db)
	if refresher != nil {
		go func() {
			if err := refresher.Start(cfg.Customers.OFACRefreshEvery); err != nil {
				cfg.Logger.Log("customers", fmt.Errorf("problem with OFAC refresher: %v", err))
			}
		}()
	}
	return refresher
}

func setupFEDClient(logger log.Logger, endpoint string, disabled string, svc *admin.Server, httpClient *http.Client) fed.Client {
	if util.Yes(disabled) {
		return nil
	}
	client := fed.NewClient(logger, endpoint, httpClient)
	if client == nil {
		panic("no FED client created")
	}
	svc.AddLivenessCheck("fed", client.Ping)
	return client
}

func setupODFIAccount(accountsClient accounts.Client, keeper *secrets.StringKeeper) *microdeposit.ODFIAccount {
	odfiAccountType := model.Savings
	if v := os.Getenv("ODFI_ACCOUNT_TYPE"); v != "" {
		t := model.AccountType(v)
		if err := t.Validate(); err == nil {
			odfiAccountType = t
		}
	}

	accountNumber := util.Or(os.Getenv("ODFI_ACCOUNT_NUMBER"), "123")
	routingNumber := util.Or(os.Getenv("ODFI_ROUTING_NUMBER"), "121042882")

	return microdeposit.NewODFIAccount(accountsClient, accountNumber, routingNumber, odfiAccountType, keeper)
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

func setupFileTransferController(
	logger log.Logger,
	controller *filetransfer.Controller,
	depRepo depository.Repository,
	fileTransferRepo config.Repository,
	microDepositRepo microdeposit.Repository,
	transferRepo transfers.Repository,
	svc *admin.Server,
) (context.CancelFunc, filetransfer.RemovalChan) {
	ctx, cancelFileSync := context.WithCancel(context.Background())

	if controller == nil {
		return cancelFileSync, nil
	}

	// setup buffered channels which only allow one concurrent operation
	flushIncoming := make(ftadmin.FlushChan, 1)
	flushOutgoing := make(ftadmin.FlushChan, 1)
	removals := make(filetransfer.RemovalChan, 1)

	// start our controller's operations in an anon goroutine
	go controller.StartPeriodicFileOperations(ctx, flushIncoming, flushOutgoing, removals)

	config.AddFileTransferConfigRoutes(logger, svc, fileTransferRepo)
	ftadmin.RegisterAdminRoutes(logger, svc, flushIncoming, flushOutgoing, func() ([]string, error) {
		return controller.GetMergedFilepaths()
	})

	return cancelFileSync, removals
}

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
	"strings"
	"syscall"
	"time"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/paygate"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/fed"
	"github.com/moov-io/paygate/internal/filetransfer"
	"github.com/moov-io/paygate/internal/microdeposit"
	"github.com/moov-io/paygate/internal/ofac"
	"github.com/moov-io/paygate/internal/util"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

var (
	httpAddr  = flag.String("http.addr", bind.HTTP("paygate"), "HTTP listen address")
	adminAddr = flag.String("admin.addr", bind.Admin("paygate"), "Admin HTTP listen address")

	flagLogFormat = flag.String("log.format", "", "Format for log lines (Options: json, plain")
)

func main() {
	flag.Parse()

	var logger log.Logger
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		*flagLogFormat = v
	}
	if strings.ToLower(*flagLogFormat) == "json" {
		logger = log.NewJSONLogger(os.Stderr)
	} else {
		logger = log.NewLogfmtLogger(os.Stderr)
	}
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	logger = log.With(logger, "caller", log.DefaultCaller)

	logger.Log("startup", fmt.Sprintf("Starting paygate server version %s", paygate.Version))

	// migrate database
	db, err := database.New(logger, os.Getenv("DATABASE_TYPE"))
	if err != nil {
		panic(fmt.Sprintf("error creating database: %v", err))
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Log("exit", err)
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
		logger.Log("admin", fmt.Sprintf("listening on %s", adminServer.BindAddr()))
		if err := adminServer.Listen(); err != nil {
			err = fmt.Errorf("problem starting admin http: %v", err)
			logger.Log("admin", err)
			errs <- err
		}
	}()
	defer adminServer.Shutdown()

	// Setup repositories
	receiverRepo := internal.NewReceiverRepo(logger, db)
	defer receiverRepo.Close()

	depositoryRepo := internal.NewDepositoryRepo(logger, db)
	defer depositoryRepo.Close()

	eventRepo := internal.NewEventRepo(logger, db)
	defer eventRepo.Close()

	gatewaysRepo := internal.NewGatewayRepo(logger, db)
	defer gatewaysRepo.Close()

	originatorsRepo := internal.NewOriginatorRepo(logger, db)
	defer originatorsRepo.Close()

	transferRepo := internal.NewTransferRepo(logger, db)
	defer transferRepo.Close()

	httpClient, err := internal.TLSHttpClient(os.Getenv("HTTP_CLIENT_CAFILE"))
	if err != nil {
		panic(fmt.Sprintf("problem creating TLS ready *http.Client: %v", err))
	}

	// Create our various Client instances
	achClient := setupACHClient(logger, adminServer, httpClient)
	fedClient := setupFEDClient(logger, adminServer, httpClient)
	ofacClient := setupOFACClient(logger, adminServer, httpClient)

	// Bring up our Accounts Client
	accountsClient := setupAccountsClient(logger, adminServer, httpClient, os.Getenv("ACCOUNTS_ENDPOINT"), os.Getenv("ACCOUNTS_CALLS_DISABLED"))
	accountsCallsDisabled := accountsClient == nil

	// Start our periodic file operations
	fileTransferRepo := filetransfer.NewRepository(logger, db, os.Getenv("DATABASE_TYPE"))
	defer fileTransferRepo.Close()
	if err := filetransfer.ValidateTemplates(fileTransferRepo); err != nil {
		panic(fmt.Sprintf("ERROR: problem validating outbound filename templates: %v", err))
	}

	achStorageDir := setupACHStorageDir(logger)
	fileTransferController, err := filetransfer.NewController(logger, achStorageDir, fileTransferRepo, achClient, accountsClient, accountsCallsDisabled)
	if err != nil {
		panic(fmt.Sprintf("ERROR: creating ACH file transfer controller: %v", err))
	}
	shutdownFileTransferController := setupFileTransferController(logger, fileTransferController, depositoryRepo, fileTransferRepo, transferRepo, adminServer)
	defer shutdownFileTransferController()

	// Register the micro-deposit admin route
	microdeposit.RegisterAdminRoutes(logger, adminServer, depositoryRepo)

	// Create HTTP handler
	handler := mux.NewRouter()
	internal.AddReceiverRoutes(logger, handler, ofacClient, receiverRepo, depositoryRepo)
	internal.AddEventRoutes(logger, handler, eventRepo)
	internal.AddGatewayRoutes(logger, handler, gatewaysRepo)
	internal.AddOriginatorRoutes(logger, handler, accountsCallsDisabled, accountsClient, ofacClient, depositoryRepo, originatorsRepo)
	internal.AddPingRoute(logger, handler)

	// Depository HTTP routes
	odfiAccount := setupODFIAccount(accountsClient)
	depositoryRouter := internal.NewDepositoryRouter(logger, odfiAccount, accountsClient, achClient, fedClient, ofacClient, depositoryRepo, eventRepo)
	depositoryRouter.RegisterRoutes(handler, accountsCallsDisabled)

	// Transfer HTTP routes
	achClientFactory := func(userId string) *achclient.ACH {
		return achclient.New(logger, userId, httpClient)
	}
	xferRouter := internal.NewTransferRouter(logger, depositoryRepo, eventRepo, receiverRepo, originatorsRepo, transferRepo, achClientFactory, accountsClient, accountsCallsDisabled)
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
			logger.Log("shutdown", err)
		}
	}
	defer shutdownServer()

	// Start main HTTP server
	go func() {
		if certFile, keyFile := os.Getenv("HTTPS_CERT_FILE"), os.Getenv("HTTPS_KEY_FILE"); certFile != "" && keyFile != "" {
			logger.Log("startup", fmt.Sprintf("binding to %s for secure HTTP server", *httpAddr))
			if err := serve.ListenAndServeTLS(certFile, keyFile); err != nil {
				logger.Log("exit", err)
			}
		} else {
			logger.Log("startup", fmt.Sprintf("binding to %s for HTTP server", *httpAddr))
			if err := serve.ListenAndServe(); err != nil {
				logger.Log("exit", err)
			}
		}
	}()

	if err := <-errs; err != nil {
		logger.Log("exit", err)
	}
}

func setupACHClient(logger log.Logger, svc *admin.Server, httpClient *http.Client) *achclient.ACH {
	client := achclient.New(logger, "ach", httpClient)
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

func setupFEDClient(logger log.Logger, svc *admin.Server, httpClient *http.Client) fed.Client {
	client := fed.NewClient(logger, httpClient)
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

func setupOFACClient(logger log.Logger, svc *admin.Server, httpClient *http.Client) ofac.Client {
	client := ofac.NewClient(logger, os.Getenv("OFAC_ENDPOINT"), httpClient)
	if client == nil {
		panic("no OFAC client created")
	}
	svc.AddLivenessCheck("ofac", client.Ping)
	return client
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

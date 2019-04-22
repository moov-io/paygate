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
	"strings"
	"syscall"
	"time"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/paygate/internal/version"
	"github.com/moov-io/paygate/pkg/achclient"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/mattn/go-sqlite3"
)

var (
	httpAddr  = flag.String("http.addr", bind.HTTP("paygate"), "HTTP listen address")
	adminAddr = flag.String("admin.addr", bind.Admin("paygate"), "Admin HTTP listen address")

	flagLogFormat = flag.String("log.format", "", "Format for log lines (Options: json, plain")
)

func main() {
	flag.Parse()

	var logger log.Logger
	if strings.ToLower(*flagLogFormat) == "json" {
		logger = log.NewJSONLogger(os.Stderr)
	} else {
		logger = log.NewLogfmtLogger(os.Stderr)
	}
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	logger = log.With(logger, "caller", log.DefaultCaller)

	logger.Log("startup", fmt.Sprintf("Starting paygate server version %s", version.Version))

	// migrate database
	if sqliteVersion, _, _ := sqlite3.Version(); sqliteVersion != "" {
		logger.Log("main", fmt.Sprintf("sqlite version %s", sqliteVersion))
	}
	db, err := createSqliteConnection(logger, getSqlitePath())
	collectDatabaseStatistics(db)
	if err != nil {
		logger.Log("main", err)
		os.Exit(1)
	}
	if err := migrate(db, logger); err != nil {
		logger.Log("main", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Log("main", err)
		}
	}()

	// Listen for application termination.
	errs := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-c)
	}()

	// Spin up admin HTTP server
	adminServer := admin.NewServer(*adminAddr)
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
	receiverRepo := &sqliteReceiverRepo{db, logger}
	defer receiverRepo.close()
	depositoryRepo := &sqliteDepositoryRepo{db, logger}
	defer depositoryRepo.close()
	eventRepo := &sqliteEventRepo{db, logger}
	defer eventRepo.close()
	gatewaysRepo := &sqliteGatewayRepo{db, logger}
	defer gatewaysRepo.close()
	originatorsRepo := &sqliteOriginatorRepo{db, logger}
	defer originatorsRepo.close()
	transferRepo := &sqliteTransferRepo{db, logger}
	defer transferRepo.close()

	// Create ACH client
	achClient := achclient.New("ach", logger)
	if achClient == nil {
		panic("no ACH client created")
	}
	adminServer.AddLivenessCheck("ach", achClient.Ping)

	// Create FED client
	fedClient := createFEDClient(logger)
	if fedClient == nil {
		panic("no FED client created")
	}
	adminServer.AddLivenessCheck("fed", fedClient.Ping)

	// Create GL client
	glClient := createGLClient(logger)
	if glClient == nil {
		panic("no GL client created")
	}
	adminServer.AddLivenessCheck("gl", glClient.Ping)

	// Create OFAC client
	ofacClient := ofacClient(logger)
	if ofacClient == nil {
		panic("no OFAC client created")
	}
	adminServer.AddLivenessCheck("ofac", ofacClient.Ping)

	// Start periodic ACH file sync
	achStorageDir := os.Getenv("ACH_FILE_STORAGE_DIR")
	if achStorageDir == "" {
		achStorageDir = "./storage/"
		os.Mkdir(achStorageDir, 0777)
	}
	fileTransferRepo := newFileTransferRepository(db)
	defer fileTransferRepo.close()
	fileTransferController, err := newFileTransferController(logger, achStorageDir, fileTransferRepo)
	if err != nil {
		panic(fmt.Sprintf("ERROR: creating ACH file transfer controller: %v", err))
	}
	ctx, cancelFileSync := context.WithCancel(context.Background())
	go fileTransferController.startPeriodicFileOperations(ctx, depositoryRepo, transferRepo)

	// Create HTTP handler
	handler := mux.NewRouter()
	addReceiverRoutes(logger, handler, ofacClient, receiverRepo, depositoryRepo)
	addDepositoryRoutes(logger, handler, fedClient, ofacClient, depositoryRepo, eventRepo)
	addEventRoutes(logger, handler, eventRepo)
	addGatewayRoutes(logger, handler, gatewaysRepo)
	addOriginatorRoutes(logger, handler, glClient, ofacClient, depositoryRepo, originatorsRepo)
	addPingRoute(logger, handler)

	xferRouter := &transferRouter{
		logger:             logger,
		depRepo:            depositoryRepo,
		eventRepo:          eventRepo,
		receiverRepository: receiverRepo,
		origRepo:           originatorsRepo,
		transferRepo:       transferRepo,

		achClientFactory: func(userId string) *achclient.ACH {
			return achclient.New(userId, logger)
		},
	}
	xferRouter.registerRoutes(handler)

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
		logger.Log("transport", "HTTP", "addr", *httpAddr)
		if err := serve.ListenAndServe(); err != nil {
			logger.Log("main", err)
		}
	}()

	if err := <-errs; err != nil {
		cancelFileSync()
		logger.Log("exit", err)
	}
	os.Exit(0)
}

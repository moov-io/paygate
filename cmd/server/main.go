// Copyright 2020 The Moov Authors
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
	"syscall"
	"time"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/database"
	"github.com/moov-io/paygate/pkg/organizations"
	"github.com/moov-io/paygate/pkg/tenants"
	"github.com/moov-io/paygate/pkg/transfers"
	transferadmin "github.com/moov-io/paygate/pkg/transfers/admin"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/offload"
	"github.com/moov-io/paygate/pkg/util"
	"github.com/moov-io/paygate/x/trace"

	"github.com/gorilla/mux"
)

var (
	flagConfigFile = flag.String("config", "", "Filepath for config file to load")
)

func main() {
	flag.Parse()

	// Read our config file
	configFilepath := util.Or(os.Getenv("CONFIG_FILE"), *flagConfigFile)
	cfg, err := config.FromFile(configFilepath)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	cfg.Logger.Log("startup", fmt.Sprintf("Starting paygate server version %s", paygate.Version))

	_, traceCloser, err := trace.NewConstantTracer(cfg.Logger, "paygate")
	if err != nil {
		panic(fmt.Sprintf("ERROR starting tracer: %v", err))
	}
	defer traceCloser.Close()

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

	// Find our fundflow strategy
	fundflowStrategy := fundflow.New()

	// Setup our transfer offloader
	transferOffloader, err := offload.New(cfg)
	if err != nil {
		panic(fmt.Sprintf("ERROR setting up transfer offloader: %v", err))
	}

	// Spin up admin HTTP server
	adminServer := admin.NewServer(cfg.Admin.BindAddress)
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

	// Create HTTP handler
	handler := mux.NewRouter()

	// Organizations
	organizationRepo := organizations.NewRepo(db)
	organizations.NewRouter(cfg.Logger, organizationRepo).RegisterRoutes(handler)

	// Tenants
	tenantsRepo := tenants.NewRepo(db)
	tenants.NewRouter(cfg.Logger, tenantsRepo).RegisterRoutes(handler)

	// Transfers
	transfersRepo := transfers.NewRepo(db)
	transfers.NewRouter(cfg.Logger, transfersRepo, fundflowStrategy, transferOffloader).RegisterRoutes(handler)
	transferadmin.RegisterRoutes(cfg.Logger, adminServer, transfersRepo)

	// Create main HTTP server
	serve := &http.Server{
		Addr:    cfg.Http.BindAddress,
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
			cfg.Logger.Log("startup", fmt.Sprintf("binding to %s for secure HTTP server", cfg.Http.BindAddress))
			if err := serve.ListenAndServeTLS(certFile, keyFile); err != nil {
				cfg.Logger.Log("exit", err)
			}
		} else {
			cfg.Logger.Log("startup", fmt.Sprintf("binding to %s for HTTP server", cfg.Http.BindAddress))
			if err := serve.ListenAndServe(); err != nil {
				cfg.Logger.Log("exit", err)
			}
		}
	}()

	if err := <-errs; err != nil {
		cfg.Logger.Log("exit", err)
	}
}

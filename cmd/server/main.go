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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/database"
	"github.com/moov-io/paygate/pkg/organizations"
	"github.com/moov-io/paygate/pkg/tenants"
	tenantadmin "github.com/moov-io/paygate/pkg/tenants/admin"
	"github.com/moov-io/paygate/pkg/transfers"
	transferadmin "github.com/moov-io/paygate/pkg/transfers/admin"
	"github.com/moov-io/paygate/pkg/transfers/fundflow"
	"github.com/moov-io/paygate/pkg/transfers/pipeline"
	"github.com/moov-io/paygate/pkg/upload"
	"github.com/moov-io/paygate/pkg/util"
	"github.com/moov-io/paygate/x/schedule"
	"github.com/moov-io/paygate/x/trace"

	"github.com/gorilla/mux"
)

var (
	flagConfigFile = flag.String("config", "", "Filepath for config file to load")
)

func main() {
	flag.Parse()

	// Read our config file
	cfg := readConfig()

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
	fundflowStrategy := fundflow.NewFirstPerson(cfg.Logger, cfg.ODFI)

	// Setup our transfer publisher
	transferPublisher, err := pipeline.NewPublisher(cfg.Pipeline)
	if err != nil {
		panic(fmt.Sprintf("ERROR setting up transfer publisher: %v", err))
	}
	defer transferPublisher.Shutdown(ctx)

	transferSubscription, err := pipeline.NewSubscription(cfg)
	if err != nil {
		panic(fmt.Sprintf("ERROR setting up transfer subscription: %v", err))
	}
	defer transferSubscription.Shutdown(ctx)

	agent, err := upload.New(cfg.Logger, "ftp", &cfg.ODFI)
	if err != nil {
		panic(fmt.Sprintf("ERROR setting up upload.Agent: %v", err))
	}
	defer agent.Close()

	merger, err := pipeline.NewMerging(cfg.Logger, cfg.Pipeline)
	if err != nil {
		panic(fmt.Sprintf("ERROR setting up xfer merging: %v", err))
	}

	cutoffs, err := schedule.ForCutoffTimes(cfg.ODFI.Cutoffs.Timezone, cfg.ODFI.Cutoffs.Windows)
	if err != nil {
		panic(fmt.Sprintf("ERROR setting up cutoff times: %v", err))
	} else {
		cfg.Logger.Log("main", fmt.Sprintf("registered %s cutoffs=%v", cfg.ODFI.Cutoffs.Timezone, strings.Join(cfg.ODFI.Cutoffs.Windows, ",")))
	}

	xferAgg := pipeline.NewAggregator(cfg.Logger, cfg.ODFI, agent, merger, transferSubscription)
	go xferAgg.Start(ctx, cutoffs)

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
	tenantadmin.RegisterRoutes(cfg.Logger, adminServer, tenantsRepo)

	// Transfers
	transfersRepo := transfers.NewRepo(db)
	defer transfersRepo.Close()
	transfers.NewRouter(cfg.Logger, transfersRepo, fundflowStrategy, transferPublisher).RegisterRoutes(handler)
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

var (
	exampleConfigFilepath = filepath.Join("examples", "config.yaml")
)

func readConfig() *config.Config {
	path := util.Or(os.Getenv("CONFIG_FILE"), *flagConfigFile, exampleConfigFilepath)
	cfg, err := config.FromFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	cfg.Logger.Log("startup", fmt.Sprintf("Starting paygate server version %s", paygate.Version))
	return cfg
}

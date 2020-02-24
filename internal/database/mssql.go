// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lopezator/migrator"
	"github.com/moov-io/base/docker"
	stdprom "github.com/prometheus/client_golang/prometheus"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/go-kit/kit/log"
	kitprom "github.com/go-kit/kit/metrics/prometheus"
	"github.com/ory/dockertest/v3"
)

var (
	mssqlDevPassword = "SecurePW(1)"

	mssqlConnections = kitprom.NewGaugeFrom(stdprom.GaugeOpts{
		Name: "mssql_connections",
		Help: "How many MSSQL connections and what status they're in.",
	}, []string{"state"})

	maxActiveMSSQLConnections = func() int {
		if v := os.Getenv("MSSQL_MAX_CONNECTIONS"); v != "" {
			if n, _ := strconv.ParseInt(v, 10, 32); n > 0 {
				return int(n)
			}
		}
		return 16
	}()

	mssqlMigrations = migrator.Migrations(
		execsql("create_test",
			`create table foo(id varchar(40) primary key);`,
		),
	)
)

type TestMSSQLDB struct {
	DB *sql.DB

	container *dockertest.Resource

	shutdown func() // context shutdown func
}

func (r *TestMSSQLDB) Close() error {
	r.shutdown()

	// Verify all connections are closed before closing DB
	if conns := r.DB.Stats().OpenConnections; conns != 0 {
		panic(fmt.Sprintf("found %d open MSSQL connections", conns))
	}

	r.container.Close()

	return r.DB.Close()
}

type mssql struct {
	dsn    string
	logger log.Logger

	connections *kitprom.Gauge
}

func (ms *mssql) Connect(ctx context.Context) (*sql.DB, error) {
	db, err := sql.Open("mssql", ms.dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxActiveMSSQLConnections)

	// Check out DB is up and working
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Migrate our database
	// if m, err := migrator.New(mssqlMigrations); err != nil {
	// 	return nil, err
	// } else {
	// 	if err := m.Migrate(db); err != nil {
	// 		return nil, err
	// 	}
	// }

	// Setup metrics after the database is setup
	go func() {
		t := time.NewTicker(1 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				stats := db.Stats()
				ms.connections.With("state", "idle").Set(float64(stats.Idle))
				ms.connections.With("state", "inuse").Set(float64(stats.InUse))
				ms.connections.With("state", "open").Set(float64(stats.OpenConnections))
			}
		}
	}()

	return db, nil
}

func mssqlConnection(logger log.Logger, user, pass string, address string, database string) *mssql {
	u := url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(user, pass),
		Host:   address,
		Path:   "",
	}

	u.Query().Set("database", database)
	u.Query().Set("connection+timeout", "30")
	u.Query().Set("encrypt", "false")
	u.Query().Set("TrustServerCertificate", "true")

	return &mssql{
		dsn:         u.String(),
		logger:      logger,
		connections: mssqlConnections,
	}
}

// renderSetupFile returns a filepath for a temporary file containing the
// database setup script to be ran on launch.
func renderSetupFile() (string, error) {
	script := []byte(`CREATE DATABASE moov;
GO
USE moov;
GO`)
	fd, err := ioutil.TempFile("", "paygate-mssql-test")
	if err != nil {
		return "", err
	}
	defer fd.Close()

	if err := ioutil.WriteFile(fd.Name(), script, 0644); err != nil {
		return "", err
	}

	return fd.Name(), nil
}

// CreateTestMSSQLDB returns a TestMSSQLDB which can be used in tests
// as a clean MS SQL database. All migrations are ran on the db before.
//
// Callers should call close on the returned *TestMSSQLDB.
func CreateTestMSSQLDB(t *testing.T) *TestMSSQLDB {
	if testing.Short() {
		t.Skip("-short flag enabled")
	}
	if !docker.Enabled() {
		t.Skip("Docker not enabled")
	}

	setupScriptFilename, err := renderSetupFile()
	if err != nil {
		t.Fatalf("problem rendering setup script: %v", err)
	}
	defer os.Remove(setupScriptFilename)

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "mcr.microsoft.com/mssql/server",
		Tag:        "2019-latest",
		Env: []string{
			"ACCEPT_EULA=Y",
			fmt.Sprintf("SA_PASSWORD=%s", mssqlDevPassword),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	logger := log.NewNopLogger()
	addr := fmt.Sprintf("localhost:%s", resource.GetPort("1433/tcp"))

	err = pool.Retry(func() error {
		db, err := mssqlConnection(logger, "sa", mssqlDevPassword, addr, "moov").Connect(context.Background())
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Ping()
	})
	if err != nil {
		resource.Close()
		t.Fatal(err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	db, err := mssqlConnection(logger, "sa", mssqlDevPassword, addr, "moov").Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Don't allow idle connections so we can verify all are closed at the end of testing
	db.SetMaxIdleConns(0)

	return &TestMSSQLDB{DB: db, container: resource, shutdown: cancelFunc}
}

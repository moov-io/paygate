// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base/docker"

	"github.com/go-kit/kit/log"
	kitprom "github.com/go-kit/kit/metrics/prometheus"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/lopezator/migrator"
	"github.com/ory/dockertest/v3"
	stdprom "github.com/prometheus/client_golang/prometheus"
)

var (
	mysqlConnections = kitprom.NewGaugeFrom(stdprom.GaugeOpts{
		Name: "mysql_connections",
		Help: "How many MySQL connections and what status they're in.",
	}, []string{"state"})

	// mySQLErrDuplicateKey is the error code for duplicate entries
	// https://dev.mysql.com/doc/refman/8.0/en/server-error-reference.html#error_er_dup_entry
	mySQLErrDuplicateKey uint16 = 1062

	maxActiveMySQLConnections = func() int {
		if v := os.Getenv("MYSQL_MAX_CONNECTIONS"); v != "" {
			if n, _ := strconv.ParseInt(v, 10, 32); n > 0 {
				return int(n)
			}
		}
		return 16
	}()

	mysqlMigrations = migrator.Migrations(
		execsql(
			"create_tenants",
			`create table tenants(tenant_id varchar(40) primary key, user_id varchar(40), name varchar(64), primary_customer varchar(40), created_at datetime, deleted_at datetime)`,
		),
		execsql(
			"create_organizations",
			`create table organizations(organization_id varchar(40) primary key, user_id varchar(40), name varchar(64), primary_customer varchar(40), created_at datetime, deleted_at datetime)`,
		),
		execsql(
			"create_tenants_organizations",
			`create table tenants_organizations(tenant_id varchar(40), organization_id varchar(40), created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_tenants_organizations_idx",
			`create unique index tenants_organizations_idx on tenants_organizations (tenant_id, organization_id);`,
		),
		execsql(
			"create_transfers",
			`create table transfers(transfer_id varchar(40) primary key, user_id varchar(40), type varchar(10), amount varchar(30), originator_id varchar(40), originator_depository varchar(40), receiver varchar(40), receiver_depository varchar(40), description varchar(200), standard_entry_class_code varchar(5), status varchar(10), same_day boolean, file_id varchar(40), transaction_id varchar(40), merged_filename varchar(100), return_code varchar(10), trace_number varchar(20), created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"add_remote_addr_to_transfers",
			// Max length for IPv6 addresses -- https://stackoverflow.com/a/7477384
			"alter table transfers add column remote_address varchar(45) default '';",
		),
	)
)

type discardLogger struct{}

func (l discardLogger) Print(v ...interface{}) {}

func init() {
	gomysql.SetLogger(discardLogger{})
}

type mysql struct {
	dsn    string
	logger log.Logger

	connections *kitprom.Gauge
}

func (my *mysql) Connect(ctx context.Context) (*sql.DB, error) {
	db, err := sql.Open("mysql", my.dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxActiveMySQLConnections)

	// Check out DB is up and working
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Migrate our database
	if m, err := migrator.New(mysqlMigrations); err != nil {
		return nil, err
	} else {
		if err := m.Migrate(db); err != nil {
			return nil, err
		}
	}

	// Setup metrics after the database is setup
	go func() {
		t := time.NewTicker(1 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				stats := db.Stats()
				my.connections.With("state", "idle").Set(float64(stats.Idle))
				my.connections.With("state", "inuse").Set(float64(stats.InUse))
				my.connections.With("state", "open").Set(float64(stats.OpenConnections))
			}
		}
	}()

	return db, nil
}

func mysqlConnection(logger log.Logger, user, pass string, address string, database string) *mysql {
	timeout := "30s"
	if v := os.Getenv("MYSQL_TIMEOUT"); v != "" {
		timeout = v
	}
	params := fmt.Sprintf("timeout=%s&charset=utf8mb4&parseTime=true&sql_mode=ALLOW_INVALID_DATES", timeout)
	dsn := fmt.Sprintf("%s:%s@%s/%s?%s", user, pass, address, database, params)
	return &mysql{
		dsn:         dsn,
		logger:      logger,
		connections: mysqlConnections,
	}
}

// TestMySQLDB is a wrapper around sql.DB for MySQL connections designed for tests to provide
// a clean database for each testcase.  Callers should cleanup with Close() when finished.
type TestMySQLDB struct {
	DB *sql.DB

	container *dockertest.Resource

	shutdown func() // context shutdown func
}

func (r *TestMySQLDB) Close() error {
	r.shutdown()

	// Verify all connections are closed before closing DB
	if conns := r.DB.Stats().OpenConnections; conns != 0 {
		panic(fmt.Sprintf("found %d open MySQL connections", conns))
	}

	r.container.Close()

	return r.DB.Close()
}

// CreateTestMySQLDB returns a TestMySQLDB which can be used in tests
// as a clean mysql database. All migrations are ran on the db before.
//
// Callers should call close on the returned *TestMySQLDB.
func CreateTestMySQLDB(t *testing.T) *TestMySQLDB {
	if testing.Short() {
		t.Skip("-short flag enabled")
	}
	if !docker.Enabled() {
		t.Skip("Docker not enabled")
	}

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "mysql",
		Tag:        "8",
		Env: []string{
			"MYSQL_USER=moov",
			"MYSQL_PASSWORD=secret",
			"MYSQL_ROOT_PASSWORD=secret",
			"MYSQL_DATABASE=paygate",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = pool.Retry(func() error {
		db, err := sql.Open("mysql", fmt.Sprintf("moov:secret@tcp(localhost:%s)/paygate", resource.GetPort("3306/tcp")))
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

	logger := log.NewNopLogger()
	address := fmt.Sprintf("tcp(localhost:%s)", resource.GetPort("3306/tcp"))

	ctx, cancelFunc := context.WithCancel(context.Background())

	db, err := mysqlConnection(logger, "moov", "secret", address, "paygate").Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Don't allow idle connections so we can verify all are closed at the end of testing
	db.SetMaxIdleConns(0)

	return &TestMySQLDB{DB: db, container: resource, shutdown: cancelFunc}
}

// MySQLUniqueViolation returns true when the provided error matches the MySQL code
// for duplicate entries (violating a unique table constraint).
func MySQLUniqueViolation(err error) bool {
	match := strings.Contains(err.Error(), fmt.Sprintf("Error %d: Duplicate entry", mySQLErrDuplicateKey))
	if e, ok := err.(*gomysql.MySQLError); ok {
		return match || e.Number == mySQLErrDuplicateKey
	}
	return match
}

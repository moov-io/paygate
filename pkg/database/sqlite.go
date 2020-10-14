// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	kitprom "github.com/go-kit/kit/metrics/prometheus"
	"github.com/lopezator/migrator"
	"github.com/mattn/go-sqlite3"
	stdprom "github.com/prometheus/client_golang/prometheus"
)

var (
	sqliteConnections = kitprom.NewGaugeFrom(stdprom.GaugeOpts{
		Name: "sqlite_connections",
		Help: "How many sqlite connections and what status they're in.",
	}, []string{"state"})

	sqliteVersionLogOnce sync.Once

	sqliteMigrations = migrator.Migrations(
		execsql(
			"create_namespace_configs",
			`create table namespace_configs(namespace primary key not null, company_identification not null)`,
		),
		execsql(
			"create_transfers",
			`create table if not exists transfers(transfer_id primary key, namespace, amount_currency, amount_value, source_customer_id, source_account_id, destination_customer_id, destination_account_id, description, status, same_day, return_code, created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"add_remote_addr_to_transfers",
			"alter table transfers add column remote_address default '';",
		),
		execsql(
			"add_micro_deposits",
			"create table micro_deposits(micro_deposit_id primary key, destination_customer_id, destination_account_id, status, return_code, created_at datetime, deleted_at datetime);",
		),
		execsql(
			"create_micro_deposits__account_id_idx",
			`create unique index micro_deposits_account_id on micro_deposits (destination_account_id);`,
		),
		execsql(
			"add_micro_deposit_amounts",
			"create table micro_deposit_amounts(micro_deposit_id, amount_currency, amount_value integer);",
		),
		execsql(
			"create_micro_deposit_transfers",
			`create table micro_deposit_transfers(micro_deposit_id, transfer_id primary key);`,
		),
		execsql(
			"create_transfer_trace_numbers",
			`create table transfer_trace_numbers(transfer_id, trace_number, unique(transfer_id, trace_number));`,
		),
		execsql(
			"add_processed_at__to__transfers",
			`alter table transfers add column processed_at datetime;`,
		),
		execsql(
			"add_processed_at__to__micro_deposits",
			`alter table micro_deposits add column processed_at datetime;`,
		),
		execsql(
			"rename_namespace_configs_to_organization_configs",
			`alter table namespace_configs rename to organization_configs;`,
		),
		execsql(
			"rename_organization_configs_namespace_to_organization",
			`alter table organization_configs rename column namespace to organization;`,
		),
		execsql(
			"rename_transfers_namespace_to_organization",
			`alter table transfers rename column namespace to organization;`,
		),
	)
)

type sqlite struct {
	path string

	connections *kitprom.Gauge
	logger      log.Logger

	err error
}

func (s *sqlite) Connect(ctx context.Context) (*sql.DB, error) {
	if s == nil {
		return nil, fmt.Errorf("nil %T", s)
	}
	if s.err != nil {
		return nil, fmt.Errorf("sqlite had error %v", s.err)
	}

	sqliteVersionLogOnce.Do(func() {
		if v, _, _ := sqlite3.Version(); v != "" {
			s.logger.Log("main", fmt.Sprintf("sqlite version %s", v))
		}
	})

	db, err := sql.Open("sqlite3", s.path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return db, err
	}

	// Migrate our database
	if m, err := migrator.New(sqliteMigrations); err != nil {
		return db, err
	} else {
		if err := m.Migrate(db); err != nil {
			return db, err
		}
	}

	// Spin up metrics only after everything works
	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				stats := db.Stats()
				s.connections.With("state", "idle").Set(float64(stats.Idle))
				s.connections.With("state", "inuse").Set(float64(stats.InUse))
				s.connections.With("state", "open").Set(float64(stats.OpenConnections))
			}

		}
	}()

	return db, err
}

func sqliteConnection(logger log.Logger, path string) *sqlite {
	if path == "" {
		return nil
	}
	return &sqlite{
		path:        path,
		logger:      logger,
		connections: sqliteConnections,
	}
}

func getSqlitePath() string {
	path := os.Getenv("SQLITE_DB_PATH")
	if path == "" || strings.Contains(path, "..") {
		// set default if empty or trying to escape
		// don't filepath.ABS to avoid full-fs reads
		path = "paygate.db"
	}
	return path
}

// TestSQLiteDB is a wrapper around sql.DB for SQLite connections designed for tests to provide
// a clean database for each testcase.  Callers should cleanup with Close() when finished.
type TestSQLiteDB struct {
	DB *sql.DB

	dir string // temp dir created for sqlite files

	shutdown func() // context shutdown func
}

func (r *TestSQLiteDB) Close() error {
	r.shutdown()

	// Verify all connections are closed before closing DB
	if conns := r.DB.Stats().OpenConnections; conns != 0 {
		panic(fmt.Sprintf("found %d open sqlite connections", conns))
	}
	if err := r.DB.Close(); err != nil {
		return err
	}
	return os.RemoveAll(r.dir)
}

// CreateTestSqliteDB returns a TestSQLiteDB which can be used in tests
// as a clean sqlite database. All migrations are ran on the db before.
//
// Callers should call close on the returned *TestSQLiteDB.
func CreateTestSqliteDB(t *testing.T) *TestSQLiteDB {
	dir, err := ioutil.TempDir("", "paygate-sqlite")
	if err != nil {
		t.Fatalf("sqlite test: %v", err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	db, err := sqliteConnection(log.NewNopLogger(), filepath.Join(dir, "paygate.db")).Connect(ctx)
	if err != nil {
		t.Fatalf("sqlite test: %v", err)
	}

	// Don't allow idle connections so we can verify all are closed at the end of testing
	db.SetMaxIdleConns(0)

	return &TestSQLiteDB{DB: db, dir: dir, shutdown: cancelFunc}
}

// SqliteUniqueViolation returns true when the provided error matches the SQLite error
// for duplicate entries (violating a unique table constraint).
func SqliteUniqueViolation(err error) bool {
	match := strings.Contains(err.Error(), "UNIQUE constraint failed")
	if e, ok := err.(sqlite3.Error); ok {
		return match || e.Code == sqlite3.ErrConstraint
	}
	return match
}

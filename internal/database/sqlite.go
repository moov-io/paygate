// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moov-io/paygate/internal/config"

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
			"create_depositories",
			`create table if not exists depositories(depository_id primary key, user_id, bank_name, holder, holder_type, type, routing_number, account_number, status, metadata, created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_micro_deposits",
			`create table if not exists micro_deposits(depository_id, user_id, amount, file_id, created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_events",
			`create table if not exists events(event_id primary key, user_id, topic, message, type, created_at datetime);`,
		),
		execsql(
			"create_gateways",
			`create table if not exists gateways(gateway_id primary key, user_id, origin, origin_name, destination, destination_name, created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_originators",
			`create table if not exists originators(originator_id primary key, user_id, default_depository, identification, metadata, created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_receivers",
			`create table if not exists receivers(receiver_id primary key, user_id, email, default_depository, status, metadata, created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_transfers",
			`create table if not exists transfers(transfer_id primary key, user_id, type, amount, originator_id, originator_depository, receiver, receiver_depository, description, standard_entry_class_code, status, same_day, file_id, transaction_id, merged_filename, return_code, trace_number, created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_cutoff_times",
			`create table if not exists cutoff_times(routing_number, cutoff, location);`,
		),
		execsql(
			"create_file_transfer_configs",
			`create table if not exists file_transfer_configs(routing_number, inbound_path, outbound_path, return_path);`,
		),
		execsql(
			"create_ftp_configs",
			`create table if not exists ftp_configs(routing_number, hostname, username, password);`,
		),
		execsql(
			"create_sftp_configs",
			`create table if not exists sftp_configs(routing_number, hostname, username, password, client_private_key, host_public_key);`,
		),
		execsql(
			"add_merged_filename_to_micro_deposits",
			"alter table micro_deposits add column merged_filename;",
		),
		execsql(
			"unique_cutoff_times",
			`create unique index cutoff_times_idx on cutoff_times(routing_number);`,
		),
		execsql(
			"unique_ftp_configs",
			`create unique index ftp_configs_idx on ftp_configs(routing_number);`,
		),
		execsql(
			"unique_sftp_configs",
			`create unique index sftp_configs_idx on sftp_configs(routing_number);`,
		),
		execsql(
			"add_return_code_to_micro_deposits",
			"alter table micro_deposits add column return_code default '';",
		),
		execsql(
			"add_transaction_id_to_micro_deposits",
			"alter table micro_deposits add column transaction_id default '';",
		),
		execsql(
			"file_transfer_configs",
			"alter table file_transfer_configs add column outbound_filename_template default '';",
		),
	)
)

type sqlite struct {
	path string

	connections *kitprom.Gauge
	logger      log.Logger

	err error
}

func (s *sqlite) Connect() (*sql.DB, error) {
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
		for range t.C {
			stats := db.Stats()
			s.connections.With("state", "idle").Set(float64(stats.Idle))
			s.connections.With("state", "inuse").Set(float64(stats.InUse))
			s.connections.With("state", "open").Set(float64(stats.OpenConnections))
		}
	}()

	return db, err
}

func sqliteConnection(logger log.Logger, path string) *sqlite {
	return &sqlite{
		path:        path,
		logger:      logger,
		connections: sqliteConnections,
	}
}

func getSqlitePath(cfg *config.Config) string {
	path := cfg.Sqlite.Path
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
}

func (r *TestSQLiteDB) Close() error {
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

	db, err := sqliteConnection(log.NewNopLogger(), filepath.Join(dir, "paygate.db")).Connect()
	if err != nil {
		t.Fatalf("sqlite test: %v", err)
	}
	return &TestSQLiteDB{db, dir}
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

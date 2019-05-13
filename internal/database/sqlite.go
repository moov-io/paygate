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
	"time"

	"github.com/go-kit/kit/log"
	kitprom "github.com/go-kit/kit/metrics/prometheus"
	"github.com/mattn/go-sqlite3"
	stdprom "github.com/prometheus/client_golang/prometheus"
)

var (
	sqliteConnections = kitprom.NewGaugeFrom(stdprom.GaugeOpts{
		Name: "sqlite_connections",
		Help: "How many sqlite connections and what status they're in.",
	}, []string{"state"})

	sqliteVersionLogOnce sync.Once
)

type sqlite struct {
	path string

	migrations  []string
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
	// Run our migrations
	for i := range s.migrations {
		res, err := db.Exec(s.migrations[i])
		if err != nil {
			return nil, fmt.Errorf("migration #%d [%s...] had problem: %v", i, s.migrations[i][:40], err)
		}
		n, err := res.RowsAffected()
		if err == nil {
			s.logger.Log("sqlite", fmt.Sprintf("migration #%d [%s...] changed %d rows", i, s.migrations[i][:40], n))
		}
	}

	// Check out DB is up and working
	if err := db.Ping(); err != nil {
		return nil, err
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

func createSqliteConnection(logger log.Logger, path string) *sqlite {
	return &sqlite{
		path:   path,
		logger: logger,
		migrations: []string{
			// Depositories
			`create table if not exists depositories(depository_id primary key, user_id, bank_name, holder, holder_type, type, routing_number, account_number, status, metadata, created_at datetime, last_updated_at datetime, deleted_at datetime);`,
			`create table if not exists micro_deposits(depository_id, user_id, amount, file_id, created_at datetime, deleted_at datetime);`,

			// Events
			`create table if not exists events(event_id primary key, user_id, topic, message, type, created_at datetime);`,

			// Gateways
			`create table if not exists gateways(gateway_id primary key, user_id, origin, origin_name, destination, destination_name, created_at datetime, deleted_at datetime);`,

			// Originators
			`create table if not exists originators(originator_id primary key, user_id, default_depository, identification, metadata, created_at datetime, last_updated_at datetime, deleted_at datetime);`,
			// Receivers
			`create table if not exists receivers(receiver_id priary key, user_id, email, default_depository, status, metadata, created_at datetime, last_updated_at datetime, deleted_at datetime);`,

			// Transfers
			`create table if not exists transfers(transfer_id, user_id, type, amount, originator_id, originator_depository, receiver, receiver_depository, description, standard_entry_class_code, status, same_day, file_id, transaction_id, merged_filename, return_code, trace_number, created_at datetime, last_updated_at datetime, deleted_at datetime);`,

			// File Merging and Uploading
			`create table if not exists cutoff_times(routing_number, cutoff, location);`,
			`create table if not exists file_transfer_configs(routing_number, inbound_path, outbound_path, return_path);`,
			// TODO(adam): sftp_configs needs the password encrypted? (or stored in vault)
			`create table if not exists sftp_configs(routing_number, hostname, username, password);`,
		},
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
func CreateTestSqliteDB() (*TestSQLiteDB, error) {
	dir, err := ioutil.TempDir("", "paygate-sqlite")
	if err != nil {
		return nil, err
	}

	db, err := createSqliteConnection(log.NewNopLogger(), filepath.Join(dir, "paygate.db")).Connect()
	if err != nil {
		return nil, err
	}
	return &TestSQLiteDB{db, dir}, nil
}

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/moov-io/base/docker"

	"github.com/go-kit/kit/log"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/ory/dockertest"
)

var (
	// mySQLErrDuplicateKey is the error code for duplicate entries
	// https://dev.mysql.com/doc/refman/8.0/en/server-error-reference.html#error_er_dup_entry
	mySQLErrDuplicateKey uint16 = 1062
)

type discardLogger struct{}

func (l discardLogger) Print(v ...interface{}) {}

func init() {
	gomysql.SetLogger(discardLogger{})
}

type mysql struct {
	dsn string

	migrations []string
	logger     log.Logger
}

func (my *mysql) Connect() (*sql.DB, error) {
	db, err := sql.Open("mysql", my.dsn)
	if err != nil {
		return nil, err
	}

	// Run our migrations
	for i := range my.migrations {
		slug := my.migrations[i]
		if len(slug) > 40 {
			slug = slug[:40]
		}
		res, err := db.Exec(my.migrations[i])
		if err != nil {
			return nil, fmt.Errorf("migration #%d [%s...] had problem: %v", i, slug, err)
		}
		n, err := res.RowsAffected()
		if err == nil {
			my.logger.Log("mysql", fmt.Sprintf("migration #%d [%s...] changed %d rows", i, slug, n))
		}
	}

	// Check out DB is up and working
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func mysqlConnection(logger log.Logger, user, pass string, address string, database string) *mysql {
	dsn := fmt.Sprintf("%s:%s@%s/%s?%s", user, pass, address, database, "timeout=30s&tls=false&charset=utf8mb4&parseTime=true&sql_mode=ALLOW_INVALID_DATES")
	return &mysql{
		dsn:    dsn,
		logger: logger,
		migrations: []string{
			// Depositories
			`create table if not exists depositories(depository_id varchar(40) primary key, user_id varchar(40), bank_name varchar(50), holder varchar(50), holder_type varchar(20), type varchar(20), routing_number varchar(10), account_number varchar(15), status varchar(20), metadata varchar(100), created_at datetime, last_updated_at datetime, deleted_at datetime);`,
			`create table if not exists micro_deposits(depository_id varchar(40), user_id varchar(40), amount varchar(10), file_id varchar(40), created_at datetime, deleted_at datetime);`,

			// Events
			`create table if not exists events(event_id varchar(40) primary key, user_id varchar(40), topic varchar(100), message varchar(250), type varchar(20), created_at datetime);`,

			// Gateways
			`create table if not exists gateways(gateway_id varchar(40) primary key, user_id varchar(40), origin varchar(10), origin_name varchar(50), destination varchar(10), destination_name varchar(50), created_at datetime, deleted_at datetime);`,

			// Originators
			`create table if not exists originators(originator_id varchar(40) primary key, user_id varchar(40), default_depository varchar(40), identification varchar(50), metadata varchar(100), created_at datetime, last_updated_at datetime, deleted_at datetime);`,

			// Receivers
			`create table if not exists receivers(receiver_id varchar(40) primary key, user_id varchar(40), email varchar(100), default_depository varchar(40), status varchar(10), metadata varchar(100), created_at datetime, last_updated_at datetime, deleted_at datetime);`,

			// Transfers
			`create table if not exists transfers(transfer_id varchar(40), user_id varchar(40), type varchar(10), amount varchar(10), originator_id varchar(40), originator_depository varchar(40), receiver varchar(40), receiver_depository varchar(40), description varchar(200), standard_entry_class_code varchar(5), status varchar(10), same_day boolean, file_id varchar(40), transaction_id varchar(40), merged_filename varchar(100), return_code varchar(10), trace_number varchar(20), created_at datetime, last_updated_at datetime, deleted_at datetime);`,

			// File Merging and Uploading
			`create table if not exists cutoff_times(routing_number varchar(10), cutoff varchar(10), location varchar(25));`,
			`create table if not exists file_transfer_configs(routing_number varchar(10), inbound_path varchar(100), outbound_path varchar(100), return_path varchar(100));`,
			`create table if not exists ftp_configs(routing_number varchar(10), hostname varchar(100), username varchar(25), password varchar(25));`,
			`create table if not exists sftp_configs(routing_number varchar(10), hostname varchar(100), username varchar(25), password varchar(25), client_private_key varchar(2100), host_public_key varchar(2100));`,
		},
	}
}

// TestMySQLDB is a wrapper around sql.DB for MySQL connections designed for tests to provide
// a clean database for each testcase.  Callers should cleanup with Close() when finished.
type TestMySQLDB struct {
	DB *sql.DB

	container *dockertest.Resource
}

func (r *TestMySQLDB) Close() error {
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

	db, err := mysqlConnection(logger, "moov", "secret", address, "paygate").Connect()
	if err != nil {
		t.Fatal(err)
	}
	return &TestMySQLDB{db, resource}
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

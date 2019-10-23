// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base/docker"
	"github.com/moov-io/paygate/internal/config"

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

	mysqlMigrations = migrator.Migrations(
		execsql(
			"create_depositories",
			`create table if not exists depositories(depository_id varchar(40) primary key, user_id varchar(40), bank_name varchar(50), holder varchar(50), holder_type varchar(20), type varchar(20), routing_number varchar(10), account_number varchar(15), status varchar(20), metadata varchar(100), created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_micro_deposits",
			`create table if not exists micro_deposits(depository_id varchar(40), user_id varchar(40), amount varchar(10), file_id varchar(40), created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_events",
			`create table if not exists events(event_id varchar(40) primary key, user_id varchar(40), topic varchar(100), message varchar(250), type varchar(20), created_at datetime);`,
		),
		execsql(
			"create_gateways",
			`create table if not exists gateways(gateway_id varchar(40) primary key, user_id varchar(40), origin varchar(10), origin_name varchar(50), destination varchar(10), destination_name varchar(50), created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_originators",
			`create table if not exists originators(originator_id varchar(40) primary key, user_id varchar(40), default_depository varchar(40), identification varchar(50), metadata varchar(100), created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_receivers",
			`create table if not exists receivers(receiver_id varchar(40) primary key, user_id varchar(40), email varchar(100), default_depository varchar(40), status varchar(10), metadata varchar(100), created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_transfers",
			`create table if not exists transfers(transfer_id varchar(40), user_id varchar(40), type varchar(10), amount varchar(10), originator_id varchar(40), originator_depository varchar(40), receiver varchar(40), receiver_depository varchar(40), description varchar(200), standard_entry_class_code varchar(5), status varchar(10), same_day boolean, file_id varchar(40), transaction_id varchar(40), merged_filename varchar(100), return_code varchar(10), trace_number varchar(20), created_at datetime, last_updated_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_cutoff_times",
			`create table if not exists cutoff_times(routing_number varchar(10), cutoff varchar(10), location varchar(25));`,
		),
		execsql(
			"create_file_transfer_configs",
			`create table if not exists file_transfer_configs(routing_number varchar(10), inbound_path varchar(100), outbound_path varchar(100), return_path varchar(100));`,
		),
		execsql(
			"create_ftp_configs",
			`create table if not exists ftp_configs(routing_number varchar(10), hostname varchar(100), username varchar(25), password varchar(25));`,
		),
		execsql(
			"create_sftp_configs",
			`create table if not exists sftp_configs(routing_number varchar(10), hostname varchar(100), username varchar(25), password varchar(25), client_private_key varchar(2100), host_public_key varchar(2100));`,
		),
		execsql(
			"add_merged_filename_to_micro_deposits",
			"alter table micro_deposits add column merged_filename varchar(100);",
		),
		execsql(
			"grow_micro_deposits_file_id",
			"alter table micro_deposits modify file_id varchar(100)",
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
			"alter table micro_deposits add column return_code varchar(10) default '';",
		),
		execsql(
			"add_transaction_id_to_micro_deposits",
			"alter table micro_deposits add column transaction_id varchar(40) default '';",
		),
		execsql(
			"file_transfer_configs",
			"alter table file_transfer_configs add column outbound_filename_template varchar(512) default '';",
		),
	)
)

type discardLogger struct{}

func (l discardLogger) Print(v ...interface{}) {}

func init() {
	gomysql.SetLogger(discardLogger{})
}

type mysql struct {
	dsn            string
	logger         log.Logger
	hostname       string
	port           int
	startupTimeout time.Duration
	connections    *kitprom.Gauge
}

func (my *mysql) Connect() (*sql.DB, error) {
	// Wait for dest port to be up and running (can take a while in docker-compose)
	if my.startupTimeout != 0 {
		address := fmt.Sprintf("%s:%d", my.hostname, my.port)
		err := WaitForConnection(address, my.startupTimeout)
		if err != nil {
			return nil, err
		}
	}

	// Open sql connection
	db, err := sql.Open("mysql", my.dsn)
	if err != nil {
		return nil, err
	}

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
		for range t.C {
			stats := db.Stats()
			my.connections.With("state", "idle").Set(float64(stats.Idle))
			my.connections.With("state", "inuse").Set(float64(stats.InUse))
			my.connections.With("state", "open").Set(float64(stats.OpenConnections))
		}
	}()

	return db, nil
}

// WaitForConnection connects to the given url with a tcp connection
// it returns nil if the connection occurs - or an err if it times out.
// You can use this to wait for a service to become available.
func WaitForConnection(host string, timeout time.Duration) error {
	// The default retry time is longish because it probably means
	// the service is not up yet.  If the service is up it will connect
	// imadiately and this value will not come into play.
	waitRetryInterval := time.Duration(5 * time.Second)

	err := doWaitForConnection(host, timeout, waitRetryInterval)

	return err
}

func doWaitForConnection(host string, timeout time.Duration, retryInterval time.Duration) error {
	connectionChan := make(chan struct{})

	startTime := time.Now()
	go func() {
		for {
			conn, err := net.DialTimeout("tcp", host, timeout)
			if err != nil {
				time.Sleep(retryInterval)
			}
			if conn != nil {
				close(connectionChan)
				break
			}
			if time.Since(startTime) > timeout {
				break
			}
		}
	}()

	select {
	case <-connectionChan:
		break
	case <-time.After(timeout):
		return errors.New("timeout error waiting for host")
	}

	return nil
}

func mysqlConnection(logger log.Logger, cfg *config.Config) *mysql {
	timeout := "30s"
	if cfg.MySQL.Timeout > 0 {
		timeout = cfg.MySQL.Timeout.String()
	}
	protocol := "tcp"
	if cfg.MySQL.Protocol != "" {
		protocol = cfg.MySQL.Protocol
	}
	port := 3306
	if cfg.MySQL.Port > 0 {
		port = cfg.MySQL.Port
	}

	params := fmt.Sprintf("timeout=%s&charset=utf8mb4&parseTime=true&sql_mode=ALLOW_INVALID_DATES", timeout)
	dsn := fmt.Sprintf("%s:%s@%s(%s:%d)/%s?%s", cfg.MySQL.User, cfg.MySQL.Password, protocol, cfg.MySQL.Hostname, port, cfg.MySQL.Database, params)
	return &mysql{
		dsn:            dsn,
		hostname:       cfg.MySQL.Hostname,
		port:           port,
		startupTimeout: cfg.MySQL.StartupTimeout,
		logger:         logger,
		connections:    mysqlConnections,
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

	cfg := config.Config{}
	cfg.MySQL.User = "moov"
	cfg.MySQL.Password = "secret"
	cfg.MySQL.Hostname = "localhost"
	cfg.MySQL.Port, _ = strconv.Atoi(resource.GetPort("3306/tcp"))
	cfg.MySQL.Database = "paygate"
	db, err := mysqlConnection(logger, &cfg).Connect()
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

// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/moov-io/base/docker"

	"github.com/go-kit/kit/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/ory/dockertest"
)

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
			my.logger.Log("sqlite", fmt.Sprintf("migration #%d [%s...] changed %d rows", i, slug, n))
		}
	}

	// Check out DB is up and working
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func mysqlConnection(logger log.Logger, user, pass string, address string, database string) *mysql {
	dsn := fmt.Sprintf("%s:%s@%s/%s?%s", user, pass, address, database, "timeout=30s&tls=false&charset=utf8mb4")
	return &mysql{
		dsn:    dsn,
		logger: logger,
		migrations: []string{
			`create table if not exists foo(id varchar(20) primary key, created_at datetime);`,
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
// as a clean sqlite database. All migrations are ran on the db before.
//
// Callers should call close on the returned *TestMySQLDB.
func CreateTestMySQLDB(t *testing.T) (*TestMySQLDB, error) {
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
		return nil, err
	}
	return &TestMySQLDB{db, resource}, nil
}

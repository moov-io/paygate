// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/moov-io/paygate/internal/config"

	"github.com/go-kit/kit/log"
)

func TestMySQL__basic(t *testing.T) {
	db := CreateTestMySQLDB(t)
	defer db.Close()

	if err := db.DB.Ping(); err != nil {
		t.Fatal(err)
	}

	// create a phony MySQL
	cfg := config.Config{}
	cfg.MySQL.User = "user"
	cfg.MySQL.Password = "password"
	cfg.MySQL.Hostname = "127.0.0.1"
	cfg.MySQL.Port = 3006
	cfg.MySQL.Database = "db"
	m := mysqlConnection(log.NewNopLogger(), &cfg)

	conn, err := m.Connect()
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	if conn != nil || err == nil {
		t.Fatalf("conn=%#v expected error", conn)
	}
}

func TestMySQLUniqueViolation(t *testing.T) {
	err := errors.New(`problem upserting depository="282f6ffcd9ba5b029afbf2b739ee826e22d9df3b", userId="f25f48968da47ef1adb5b6531a1c2197295678ce": Error 1062: Duplicate entry '282f6ffcd9ba5b029afbf2b739ee826e22d9df3b' for key 'PRIMARY'`)
	if !UniqueViolation(err) {
		t.Error("should have matched unique violation")
	}
}

func TestWaitForConnection(t *testing.T) {
	start := time.Now()
	err := WaitForConnection("localhost:8884", 100*time.Millisecond)
	elapsedTime := time.Since(start)
	if err.Error() != "timeout error waiting for host" {
		t.Errorf("error msg does not match: %s", err.Error())
	}
	if elapsedTime < 100*time.Millisecond || elapsedTime > 120*time.Millisecond {
		t.Errorf("elapsed time not in window: %d", elapsedTime)
	}
}

func TestWaitForConnectionWithConnection(t *testing.T) {
	go func() {
		l, _ := net.Listen("tcp", "localhost:8886")
		defer l.Close()

		conn, _ := l.Accept()
		defer conn.Close()
	}()

	err := doWaitForConnection("localhost:8886", 100*time.Millisecond, 10*time.Millisecond)
	if err != nil {
		t.Errorf("unexpected err: %v", err)
	}
}

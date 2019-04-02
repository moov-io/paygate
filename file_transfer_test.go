// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	filedriver "github.com/goftp/file-driver"
	"github.com/goftp/server"
	"github.com/jlaffaye/ftp"
)

func createTestSFTPServer(t *testing.T) *server.Server {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
	opts := &server.ServerOpts{
		Auth: &server.SimpleAuth{
			Name:     "moov",
			Password: "password",
		},
		Factory: &filedriver.FileDriverFactory{
			RootPath: "./testdata/ftp-server",
			Perm:     server.NewSimplePerm("test", "test"),
		},
		Hostname: "localhost",
		Port:     2023,
	}
	svc := server.NewServer(opts)
	if svc == nil {
		t.Fatal("nil FTP server")
	}
	go svc.ListenAndServe()
	return svc
}

func createTestFTPConnection(t *testing.T) *ftp.ServerConn {
	conn, err := ftp.DialTimeout("localhost:2023", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn.Login("moov", "password")
	return conn
}

func TestSFTP_ping(t *testing.T) {
	svc := createTestSFTPServer(t)
	defer svc.Shutdown()

	conn := createTestFTPConnection(t)
	defer conn.Quit()

	dir, err := conn.CurrentDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Error("empty current dir?!")
	}

	// Change directory
	if err := conn.ChangeDir("scratch"); err != nil {
		t.Error(err)
	}

	// Read a file we know should exist
	resp, err := conn.RetrFrom("existing-file", 0) // offset of 0
	if err != nil {
		t.Error(err)
	}
	bs, _ := ioutil.ReadAll(resp)
	bs = bytes.TrimSpace(bs)
	if !bytes.Equal(bs, []byte("Hello, World!")) {
		t.Errorf("got %q", string(bs))
	}
}

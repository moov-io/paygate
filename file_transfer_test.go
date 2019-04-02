// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	filedriver "github.com/goftp/file-driver"
	"github.com/goftp/server"
	"github.com/jlaffaye/ftp"
)

var (
	portSource = rand.NewSource(time.Now().Unix())
)

func port() int {
	return int(30000 + (portSource.Int63() % 9999))
}

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
		Port:     port(),
		Logger:   &server.DiscardLogger{},
	}
	svc := server.NewServer(opts)
	if svc == nil {
		t.Fatal("nil FTP server")
	}
	go svc.ListenAndServe()
	return svc
}

func createTestFTPConnection(t *testing.T, svc *server.Server) *ftp.ServerConn {
	conn, err := ftp.DialTimeout(fmt.Sprintf("localhost:%d", svc.Port), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn.Login("moov", "password")
	return conn
}

func TestSFTP(t *testing.T) {
	svc := createTestSFTPServer(t)
	defer svc.Shutdown()

	conn := createTestFTPConnection(t, svc)
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

func createTestFileTransferAgent(t *testing.T) (*server.Server, *FileTransferAgent) {
	svc := createTestSFTPServer(t)

	auth, ok := svc.Auth.(*server.SimpleAuth)
	if !ok {
		t.Errorf("unknown svc.Auth: %T", svc.Auth)
	}
	sftpConf := &SFTPConfig{
		Hostname: fmt.Sprintf("%s:%d", svc.Hostname, svc.Port),
		Username: auth.Name,
		Password: auth.Password,
	}
	conf := &FileTransferConfig{ // these need to match paths at testdata/ftp-srever/
		InboundPath:  "inbound",
		OutboundPath: "outgoing",
		ReturnPath:   "returned",
	}
	agent, err := NewFileTransfer(sftpConf, conf)
	if err != nil {
		svc.Shutdown()
		t.Fatalf("problem creating FileTransferAgent: %v", err)
	}
	return svc, agent
}

func TestSFTP__getInboundFiles(t *testing.T) {
	svc, agent := createTestFileTransferAgent(t)
	defer svc.Shutdown()

	files, err := agent.getInboundFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files", len(files))
	}
	if files[0].filename != "transfer.ach" {
		t.Errorf("files[0]=%s", files[0])
	}
	bs, _ := ioutil.ReadAll(files[0].contents)
	bs = bytes.TrimSpace(bs)
	if !bytes.Equal(bs, []byte("test ACH file")) {
		t.Errorf("got %v", string(bs))
	}

	// make sure we perform the same call and get the same result
	files, err = agent.getInboundFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files", len(files))
	}
	if files[0].filename != "transfer.ach" {
		t.Errorf("files[0]=%s", files[0])
	}
}

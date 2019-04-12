// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"

	filedriver "github.com/goftp/file-driver"
	"github.com/goftp/server"
	"github.com/jlaffaye/ftp"
)

var (
	portSource = rand.NewSource(time.Now().Unix())
)

type mockFileTransferAgent struct {
	inboundFiles []file
	returnFiles  []file
	uploadedFile *file  // non-nil on file upload
	deletedFile  string // filepath of last deleted file
}

func (a *mockFileTransferAgent) getInboundFiles() ([]file, error) {
	return a.inboundFiles, nil
}

func (a *mockFileTransferAgent) getReturnFiles() ([]file, error) {
	return a.returnFiles, nil
}

func (a *mockFileTransferAgent) uploadFile(f file) error {
	// read f.contents before callers close the underlying os.Open file descriptor
	bs, _ := ioutil.ReadAll(f.contents)
	a.uploadedFile = &f
	a.uploadedFile.contents = ioutil.NopCloser(bytes.NewReader(bs))
	return nil
}

func (a *mockFileTransferAgent) delete(path string) error {
	a.deletedFile = path
	return nil
}

func (a *mockFileTransferAgent) inboundPath() string  { return "inbound/" }
func (a *mockFileTransferAgent) outboundPath() string { return "outbound/" }
func (a *mockFileTransferAgent) returnPath() string   { return "return/" }
func (a *mockFileTransferAgent) close() error         { return nil }

func port() int {
	return int(30000 + (portSource.Int63() % 9999))
}

func createTestSFTPServer(t *testing.T) (*server.Server, error) {
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
			RootPath: filepath.Join("testdata", "ftp-server"),
			Perm:     server.NewSimplePerm("test", "test"),
		},
		Hostname: "localhost",
		Port:     port(),
		Logger:   &server.DiscardLogger{},
	}
	svc := server.NewServer(opts)
	if svc == nil {
		return nil, errors.New("nil FTP server")
	}
	if err := try(func() error { return svc.ListenAndServe() }, 50*time.Millisecond); err != nil {
		if err == errTimeout {
			return svc, nil
		}
		return nil, err
	}
	return svc, nil
}

func createTestFTPConnection(t *testing.T, svc *server.Server) (*ftp.ServerConn, error) {
	t.Helper()
	conn, err := ftp.DialTimeout(fmt.Sprintf("localhost:%d", svc.Port), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Login("moov", "password"); err != nil {
		t.Fatal(err)
	}
	return conn, nil
}

func TestSFTP(t *testing.T) {
	svc, err := createTestSFTPServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Shutdown()

	conn, err := createTestFTPConnection(t, svc)
	if err != nil {
		t.Fatal(err)
	}
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

func createTestFileTransferAgent(t *testing.T) (*server.Server, fileTransferAgent) {
	svc, err := createTestSFTPServer(t)
	if err != nil {
		return nil, nil
	}

	auth, ok := svc.Auth.(*server.SimpleAuth)
	if !ok {
		t.Errorf("unknown svc.Auth: %T", svc.Auth)
	}
	sftpConf := &sftpConfig{
		Hostname: fmt.Sprintf("%s:%d", svc.Hostname, svc.Port),
		Username: auth.Name,
		Password: auth.Password,
	}
	conf := &fileTransferConfig{ // these need to match paths at testdata/ftp-srever/
		InboundPath:  "inbound",
		OutboundPath: "outbound",
		ReturnPath:   "returned",
	}
	agent, err := newFileTransferAgent(sftpConf, conf)
	if err != nil {
		svc.Shutdown()
		t.Fatalf("problem creating FileTransferAgent: %v", err)
		return nil, nil
	}
	return svc, agent
}

func TestSFTP__getInboundFiles(t *testing.T) {
	svc, agent := createTestFileTransferAgent(t)
	defer agent.close()
	defer svc.Shutdown()

	files, err := agent.getInboundFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files", len(files))
	}
	if files[0].filename != "iat-credit.ach" {
		t.Errorf("files[0]=%s", files[0])
	}
	bs, _ := ioutil.ReadAll(files[0].contents)
	bs = bytes.TrimSpace(bs)
	if !strings.HasPrefix(string(bs), "101 121042882 2313801041812180000A094101Bank                   My Bank Name                   ") {
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
	if files[0].filename != "iat-credit.ach" {
		t.Errorf("files[0]=%s", files[0])
	}
}

func TestSFTP__getReturnFiles(t *testing.T) {
	svc, agent := createTestFileTransferAgent(t)
	defer agent.close()
	defer svc.Shutdown()

	files, err := agent.getReturnFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files", len(files))
	}
	if files[0].filename != "return-WEB.ach" {
		t.Errorf("files[0]=%s", files[0])
	}
	bs, _ := ioutil.ReadAll(files[0].contents)
	bs = bytes.TrimSpace(bs)
	if !strings.HasPrefix(string(bs), "101 091400606 6910001341810170306A094101FIRST BANK & TRUST     ASF APPLICATION SUPERVI        ") {
		t.Errorf("got %v", string(bs))
	}

	// make sure we perform the same call and get the same result
	files, err = agent.getReturnFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files", len(files))
	}
	if files[0].filename != "return-WEB.ach" {
		t.Errorf("files[0]=%s", files[0])
	}
}

func TestSFTP__uploadFile(t *testing.T) {
	svc, agent := createTestFileTransferAgent(t)
	defer agent.close()
	defer svc.Shutdown()

	content := base.ID()
	f := file{
		filename: base.ID(),
		contents: ioutil.NopCloser(strings.NewReader(content)), // random file contents
	}

	// Create outbound directory
	parent := filepath.Join("testdata", "ftp-server", agent.outboundPath())
	os.Mkdir(parent, 0777)
	defer os.Remove(filepath.Join("testdata", "ftp-server", agent.outboundPath(), f.filename))

	if err := agent.uploadFile(f); err != nil {
		t.Fatal(err)
	}

	ftpAgent, _ := agent.(*ftpFileTransferAgent)

	// manually read file contents
	ftpAgent.conn.ChangeDir(agent.outboundPath())
	resp, _ := ftpAgent.conn.Retr(f.filename)
	if resp == nil {
		t.Fatal("nil File response")
	}
	r, _ := ftpAgent.readResponse(resp)
	if r == nil {
		t.Fatal("failed to read file")
	}
	bs, _ := ioutil.ReadAll(r)
	if !bytes.Equal(bs, []byte(content)) {
		t.Errorf("got %q", string(bs))
	}
}

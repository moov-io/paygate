// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

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
	"github.com/moov-io/paygate/internal/config"
	mhttptest "github.com/moov-io/paygate/internal/httptest"
	"github.com/moov-io/paygate/internal/util"

	"github.com/go-kit/kit/log"
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

func createTestFTPServer(t *testing.T) (*server.Server, error) {
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
			RootPath: filepath.Join("..", "..", "testdata", "ftp-server"),
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
	if err := util.Timeout(func() error { return svc.ListenAndServe() }, 50*time.Millisecond); err != nil {
		if err == util.ErrTimeout {
			return svc, nil
		}
		return nil, err
	}
	return svc, nil
}

func TestFTPConfig__String(t *testing.T) {
	cfg := &FTPConfig{"routing", "host", "user", "pass"}
	if !strings.Contains(cfg.String(), "Password=p**s") {
		t.Error(cfg.String())
	}
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

func TestFTP(t *testing.T) {
	svc, err := createTestFTPServer(t)
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

func createTestFTPAgent(t *testing.T) (*server.Server, *FTPTransferAgent) {
	svc, err := createTestFTPServer(t)
	if err != nil {
		return nil, nil
	}

	auth, ok := svc.Auth.(*server.SimpleAuth)
	if !ok {
		t.Errorf("unknown svc.Auth: %T", svc.Auth)
	}
	conf := &Config{ // these need to match paths at testdata/ftp-srever/
		InboundPath:  "inbound",
		OutboundPath: "outbound",
		ReturnPath:   "returned",
	}
	ftpConfigs := []*FTPConfig{
		{
			Hostname: fmt.Sprintf("%s:%d", svc.Hostname, svc.Port),
			Username: auth.Name,
			Password: auth.Password,
		},
	}
	agent, err := newFTPTransferAgent(log.NewNopLogger(), config.Empty(), conf, ftpConfigs)
	if err != nil {
		svc.Shutdown()
		t.Fatalf("problem creating FileTransferAgent: %v", err)
		return nil, nil
	}
	return svc, agent
}

func TestFTPAgent(t *testing.T) {
	svc, agent := createTestFTPAgent(t)
	defer agent.Close()
	defer svc.Shutdown()

	// Verify directories aren setup as expected
	if v := agent.InboundPath(); v != "inbound" {
		t.Errorf("got %s", v)
	}
	if v := agent.OutboundPath(); v != "outbound" {
		t.Errorf("got %s", v)
	}
	if v := agent.ReturnPath(); v != "returned" {
		t.Errorf("got %s", v)
	}
}

func TestFTP__tlsDialOption(t *testing.T) {
	if testing.Short() {
		return // skip network calls
	}

	cafile, err := mhttptest.GrabConnectionCertificates(t, "google.com:443")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(cafile)

	opt, err := tlsDialOption(cafile)
	if err != nil {
		t.Fatal(err)
	}
	if opt == nil {
		t.Fatal("nil tls DialOption")
	}
}

func TestFTP__getInboundFiles(t *testing.T) {
	svc, agent := createTestFTPAgent(t)
	defer agent.Close()
	defer svc.Shutdown()

	files, err := agent.GetInboundFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files", len(files))
	}
	for i := range files {
		if files[i].Filename == "iat-credit.ach" {
			bs, _ := ioutil.ReadAll(files[i].Contents)
			bs = bytes.TrimSpace(bs)
			if !strings.HasPrefix(string(bs), "101 121042882 2313801041812180000A094101Bank                   My Bank Name                   ") {
				t.Errorf("got %v", string(bs))
			}
		}
	}

	// make sure we perform the same call and get the same result
	files, err = agent.GetInboundFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files", len(files))
	}
	for i := range files {
		if files[0].Filename == "iat-credit.ach" {
			continue
		}
		if files[0].Filename == "cor-c01.ach" {
			continue
		}
		t.Errorf("files[%d]=%s", i, files[i])
	}
}

func TestFTP__getReturnFiles(t *testing.T) {
	svc, agent := createTestFTPAgent(t)
	defer agent.Close()
	defer svc.Shutdown()

	files, err := agent.GetReturnFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files", len(files))
	}
	if files[0].Filename != "return-WEB.ach" {
		t.Errorf("files[0]=%s", files[0])
	}
	bs, _ := ioutil.ReadAll(files[0].Contents)
	bs = bytes.TrimSpace(bs)
	if !strings.HasPrefix(string(bs), "101 091400606 6910001341810170306A094101FIRST BANK & TRUST     ASF APPLICATION SUPERVI        ") {
		t.Errorf("got %v", string(bs))
	}

	// make sure we perform the same call and get the same result
	files, err = agent.GetReturnFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files", len(files))
	}
	if files[0].Filename != "return-WEB.ach" {
		t.Errorf("files[0]=%s", files[0])
	}
}

func TestFTP__uploadFile(t *testing.T) {
	svc, agent := createTestFTPAgent(t)
	defer agent.Close()
	defer svc.Shutdown()

	content := base.ID()
	f := File{
		Filename: base.ID(),
		Contents: ioutil.NopCloser(strings.NewReader(content)), // random file contents
	}

	// Create outbound directory
	parent := filepath.Join("..", "..", "testdata", "ftp-server", agent.OutboundPath())
	os.Mkdir(parent, 0777)

	if err := agent.UploadFile(f); err != nil {
		t.Fatal(err)
	}

	// manually read file contents
	agent.conn.ChangeDir(agent.OutboundPath())
	resp, _ := agent.conn.Retr(f.Filename)
	if resp == nil {
		t.Fatal("nil File response")
	}
	r, _ := agent.readResponse(resp)
	if r == nil {
		t.Fatal("failed to read file")
	}
	bs, _ := ioutil.ReadAll(r)
	if !bytes.Equal(bs, []byte(content)) {
		t.Errorf("got %q", string(bs))
	}

	// delete the file
	if err := agent.Delete(f.Filename); err != nil {
		t.Fatal(err)
	}

	// get an error with no FTP configs
	agent.ftpConfigs = nil
	if err := agent.UploadFile(f); err == nil {
		t.Error("expected error")
	}
}

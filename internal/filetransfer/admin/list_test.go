// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/moov-io/ach"
	"github.com/moov-io/base/admin"
)

var (
	getZeroFiles = func() ([]string, error) {
		return nil, nil
	}
)

func TestAdmin__listMergedFiles(t *testing.T) {
	getter := setupTestDir(t)

	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	flushOutgoing := make(FlushChan, 1)
	RegisterAdminRoutes(log.NewNopLogger(), svc, nil, flushOutgoing, getter)

	body := Get(t, "http://"+svc.BindAddr()+"/files/merged")
	defer body.Close()

	var resp mergedFiles
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if n := len(resp.Files); n != 1 {
		t.Errorf("got %d files: %#v", n, resp.Files)
	}

	file := resp.Files[0]
	if file.Filename != "test.ach" {
		t.Errorf("file.Filename=%s", file.Filename)
	}
	if file.Header == nil {
		t.Error("nil FileHeader")
	}
}

func TestAdmin__getMergedFile(t *testing.T) {
	getter := setupTestDir(t)

	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	flushOutgoing := make(FlushChan, 1)
	RegisterAdminRoutes(log.NewNopLogger(), svc, nil, flushOutgoing, getter)

	body := Get(t, "http://"+svc.BindAddr()+"/files/merged/test.ach")
	defer body.Close()

	bs, _ := ioutil.ReadAll(body)
	file, err := ach.FileFromJSON(bs)
	if err != nil {
		t.Fatal(err)
	}
	if file.Header.ImmediateOrigin != "076401251" {
		t.Errorf("origin=%s", file.Header.ImmediateOrigin)
	}
}

func setupTestDir(t *testing.T) func() ([]string, error) {
	dir, err := ioutil.TempDir("", "admin-list-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	in, err := os.Open(filepath.Join("..", "..", "..", "testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	out, err := os.Create(filepath.Join(dir, "test.ach"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}

	return func() ([]string, error) {
		return filepath.Glob(filepath.Join(dir, "*.ach"))
	}
}

func Get(t *testing.T, url string) io.ReadCloser {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bogus HTTP status: %d", resp.StatusCode)
	}
	return resp.Body
}

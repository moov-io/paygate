// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/testclient"
)

func TestConfigRoute(t *testing.T) {
	cfg, err := config.FromFile(filepath.Join("..", "testdata", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	svc, _ := testclient.Admin(t)
	RegisterRoutes(svc, cfg)

	resp, err := http.DefaultClient.Get("http://" + svc.BindAddr() + "/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("bogus HTTP status: %s", resp.Status)
	}

	bs, _ := ioutil.ReadAll(resp.Body)
	body := string(bs)
	t.Log(body)

	if strings.Contains(body, "secret") {
		t.Fatal("FTP password was not masked")
	}
	if strings.Contains(body, "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=") {
		t.Fatal("encryption keyURI was not masked")
	}

	var got config.Config
	if err := json.Unmarshal(bs, &got); err != nil {
		t.Fatal(err)
	}
	if got.ODFI.FTP == nil || !strings.Contains(got.ODFI.FTP.Password, "*") {
		t.Fatalf("expected masked FTP password, got %#v", got.ODFI.FTP)
	}
	if cfg.ODFI.FTP.Password != "secret" {
		t.Fatalf("handler should not mutate the live config, got password %q", cfg.ODFI.FTP.Password)
	}
}

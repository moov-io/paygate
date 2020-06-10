// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"io/ioutil"
	"net/http"
	"path/filepath"
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

	if _, err := config.Read(bs); err != nil {
		t.Fatal(err)
	}
}

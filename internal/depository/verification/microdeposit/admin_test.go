// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/moov-io/base"
	moovadmin "github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/admin"
	"github.com/moov-io/paygate/internal/model"

	"github.com/go-kit/kit/log"
)

func TestMicroDeposits__AdminGetMicroDeposits(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	amt1, _ := model.NewAmount("USD", "0.11")
	amt2, _ := model.NewAmount("USD", "0.32")

	repo := &MockRepository{
		MicroDeposits: []*MicroDeposit{
			{Amount: *amt1},
			{Amount: *amt2},
		},
	}
	RegisterAdminRoutes(log.NewNopLogger(), svc, repo)

	req, err := http.NewRequest("GET", "http://"+svc.BindAddr()+"/depositories/foo/micro-deposits", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("bogus HTTP status: %s", resp.Status)
	}

	defer resp.Body.Close()

	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	t.Log(string(bytes.TrimSpace(bs)))

	type response struct {
		Amount model.Amount `json:"amount"`
	}
	var rs []response
	if err := json.NewDecoder(bytes.NewReader(bs)).Decode(&rs); err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Errorf("got %d micro-deposits", len(rs))
	}
	for i := range rs {
		switch v := rs[i].Amount.String(); v {
		case "USD 0.11", "USD 0.32":
			t.Logf("matched %s", v)
		default:
			t.Errorf("got %s", v)
		}
	}

	// bad case, Repository returns an error
	repo.Err = errors.New("bad error")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %s", resp.Status)
	}
}

func TestMicroDeposits__adminRead(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	amt1, _ := model.NewAmount("USD", "0.11")
	amt2, _ := model.NewAmount("USD", "0.32")

	depRepo := &MockRepository{
		MicroDeposits: []*MicroDeposit{
			{Amount: *amt1},
			{Amount: *amt2},
		},
	}
	RegisterAdminRoutes(log.NewNopLogger(), svc, depRepo)

	conf := admin.NewConfiguration()
	conf.Host = svc.BindAddr()
	client := admin.NewAPIClient(conf)

	amounts, resp, err := client.AdminApi.GetMicroDeposits(context.Background(), base.ID())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected HTTP status: %v", resp.Status)
	}

	if len(amounts) != 2 {
		t.Errorf("unexpected %d micro-deposit amounts", len(amounts))
	}
}

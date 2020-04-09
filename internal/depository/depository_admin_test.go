// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"
	moovadmin "github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/admin"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

func TestDepository__overrideDepositoryStatus(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	depID, userID := base.ID(), id.User(base.ID())

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	keeper := secrets.TestStringKeeper(t)

	repo := NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper)

	if err := repo.UpsertUserDepository(userID, &model.Depository{
		ID:            id.Depository(depID),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    model.Individual,
		Type:          model.Checking,
		RoutingNumber: "123",
		Status:        model.DepositoryUnverified,
		Created:       base.NewTime(time.Now().Add(-1 * time.Second)),
	}); err != nil {
		t.Fatal(err)
	}

	RegisterAdminRoutes(log.NewNopLogger(), svc, repo)

	addr := fmt.Sprintf("http://%s/depositories/%s", svc.BindAddr(), depID)
	body := strings.NewReader(`{"status": "rejected"}`)

	req, _ := http.NewRequest("PUT", addr, body)
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %s: %v", resp.Status, string(bs))
	}

	dep, err := repo.GetUserDepository(id.Depository(depID), userID)
	if err != nil {
		t.Fatal(err)
	}
	if dep.Status != model.DepositoryRejected {
		t.Errorf("dep.Status=%v", dep.Status)
	}
}

func TestDepository__overrideDepositoryStatusErr(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := &MockRepository{
		Err: errors.New("bad error"),
	}

	RegisterAdminRoutes(log.NewNopLogger(), svc, repo)

	depID := base.ID()
	addr := fmt.Sprintf("http://%s/depositories/%s", svc.BindAddr(), depID)
	body := strings.NewReader(`{"status": "rejected"}`)

	req, _ := http.NewRequest("PUT", addr, body)
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %s: %v", resp.Status, string(bs))
	}

	// invalid route
	resp, _ = http.DefaultClient.Get(addr)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %s", resp.Status)
	}
}

func TestDepository__adminStatusUpdate(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	depID, userID := base.ID(), id.User(base.ID())

	keeper := secrets.TestStringKeeper(t)
	repo := NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)

	now := base.NewTime(time.Now())
	dep := &model.Depository{
		ID:            id.Depository(depID),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    model.Individual,
		Type:          model.Checking,
		RoutingNumber: "121421212",
		Status:        model.DepositoryUnverified,
		Metadata:      "metadata",
		Created:       now,
		Updated:       now,
		Keeper:        keeper,
	}
	if err := dep.ReplaceAccountNumber("1234"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	RegisterAdminRoutes(log.NewNopLogger(), svc, repo)

	conf := admin.NewConfiguration()
	conf.Host = svc.BindAddr()
	client := admin.NewAPIClient(conf)

	out, resp, err := client.AdminApi.UpdateDepositoryStatus(context.Background(), depID, admin.UpdateDepository{
		Status: admin.VERIFIED,
	})

	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("unexpected HTTP status: %v: %s", resp.Status, string(bs))
	}
	if out.Status != admin.VERIFIED {
		t.Errorf("got depository status: %v", out.Status)
	}
}

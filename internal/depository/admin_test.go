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
	"github.com/moov-io/paygate/internal"
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

	repo := internal.NewDepositoryRepo(log.NewNopLogger(), sqliteDB.DB, keeper)

	if err := repo.UpsertUserDepository(userID, &internal.Depository{
		ID:            id.Depository(depID),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    internal.Individual,
		Type:          model.Checking,
		RoutingNumber: "123",
		Status:        internal.DepositoryUnverified,
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
	if dep.Status != internal.DepositoryRejected {
		t.Errorf("dep.Status=%v", dep.Status)
	}
}

func TestDepository__overrideDepositoryStatusErr(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := &internal.MockDepositoryRepository{
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

	depID := base.ID()
	repo := &internal.MockDepositoryRepository{
		Depositories: []*internal.Depository{
			{
				ID: id.Depository(depID),
			},
		},
	}

	RegisterAdminRoutes(log.NewNopLogger(), svc, repo)

	conf := admin.NewConfiguration()
	conf.Host = svc.BindAddr()
	client := admin.NewAPIClient(conf)

	resp, err := client.AdminApi.UpdateDepositoryStatus(context.Background(), depID, admin.UpdateDepository{
		Status: "verified",
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected HTTP status: %v", resp.Status)
	}
}

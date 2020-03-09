// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

type approvalTest struct {
	svc *admin.Server
	db  *database.TestSQLiteDB

	userID id.User
	repo   Repository
}

func (at *approvalTest) close() {
	at.svc.Shutdown()
	at.db.Close()
}

func setupApprovalTest(t *testing.T) *approvalTest {
	svc := admin.NewServer(":0")
	go svc.Listen()

	sqliteDB := database.CreateTestSqliteDB(t)

	userID := id.User(base.ID())
	repo := NewTransferRepo(log.NewNopLogger(), sqliteDB.DB)

	RegisterAdminRoutes(log.NewNopLogger(), svc, repo)

	return &approvalTest{
		svc:    svc,
		db:     sqliteDB,
		userID: userID,
		repo:   repo,
	}
}

func TestApproval__Reviewable(t *testing.T) {
	test := setupApprovalTest(t)
	defer test.close()

	// missing Transfer
	body := `{"status":"pending"}`
	req, _ := http.NewRequest("PUT", "http://"+test.svc.BindAddr()+fmt.Sprintf("/users/%s/transfers/id/status", test.userID), strings.NewReader(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %v", resp.Status)
	}

	// write the transfer
	amt, _ := model.NewAmount("USD", "14.22")
	xfers, err := test.repo.createUserTransfers(test.userID, []*transferRequest{
		{
			Type:                   model.PushTransfer,
			Amount:                 *amt,
			Originator:             model.OriginatorID(base.ID()),
			OriginatorDepository:   id.Depository(base.ID()),
			Receiver:               model.ReceiverID(base.ID()),
			ReceiverDepository:     id.Depository(base.ID()),
			Description:            "example",
			StandardEntryClassCode: "PPD",
			userID:                 test.userID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(xfers) != 1 {
		t.Fatalf("xfers=%#v", xfers)
	}

	// Update transfer into Reviewable
	if err := test.repo.UpdateTransferStatus(xfers[0].ID, model.TransferReviewable); err != nil {
		t.Fatalf("xfers[0]=%#v", xfers[0])
	}

	// try, but with invalid transition
	body = `{"status": "reclaimed"}`
	req, _ = http.NewRequest("PUT", "http://"+test.svc.BindAddr()+fmt.Sprintf("/users/%s/transfers/%s/status", test.userID, xfers[0].ID), strings.NewReader(body))

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %v: %s", resp.Status, string(bs))
	}

	// retry request now that it's setup properly
	body = `{"status":"pending"}`
	req, _ = http.NewRequest("PUT", "http://"+test.svc.BindAddr()+fmt.Sprintf("/users/%s/transfers/%s/status", test.userID, xfers[0].ID), strings.NewReader(body))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %v: %v", resp.Status, string(bs))
	}

	// attempt update with invalid status transition
	body = `{"status": "reclaimed"}`
	req, _ = http.NewRequest("PUT", "http://"+test.svc.BindAddr()+fmt.Sprintf("/users/%s/transfers/%s/status", test.userID, xfers[0].ID), strings.NewReader(body))

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %v: %s", resp.Status, string(bs))
	}
}

func TestApproval__Pending(t *testing.T) {
	test := setupApprovalTest(t)
	defer test.close()

	// write the transfer
	amt, _ := model.NewAmount("USD", "14.22")
	xfers, err := test.repo.createUserTransfers(test.userID, []*transferRequest{
		{
			Type:                   model.PushTransfer,
			Amount:                 *amt,
			Originator:             model.OriginatorID(base.ID()),
			OriginatorDepository:   id.Depository(base.ID()),
			Receiver:               model.ReceiverID(base.ID()),
			ReceiverDepository:     id.Depository(base.ID()),
			Description:            "example",
			StandardEntryClassCode: "PPD",
			userID:                 test.userID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(xfers) != 1 {
		t.Fatalf("xfers=%#v", xfers)
	}

	// Update transfer into Reviewable
	if err := test.repo.UpdateTransferStatus(xfers[0].ID, model.TransferPending); err != nil {
		t.Fatalf("xfers[0]=%#v", xfers[0])
	}

	// perform status update
	body := `{"status": "canceled"}`
	req, _ := http.NewRequest("PUT", "http://"+test.svc.BindAddr()+fmt.Sprintf("/users/%s/transfers/%s/status", test.userID, xfers[0].ID), strings.NewReader(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %v: %s", resp.Status, string(bs))
	}

	// attempt update with invalid status transition
	body = `{"status": "reclaimed"}`
	req, _ = http.NewRequest("PUT", "http://"+test.svc.BindAddr()+fmt.Sprintf("/users/%s/transfers/%s/status", test.userID, xfers[0].ID), strings.NewReader(body))

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %v: %s", resp.Status, string(bs))
	}
}

func TestApproval__Errors(t *testing.T) {
	test := setupApprovalTest(t)
	defer test.close()

	// missing Transfer
	body := `{...}` // invalid json
	req, _ := http.NewRequest("PUT", "http://"+test.svc.BindAddr()+fmt.Sprintf("/users/%s/transfers/id/status", test.userID), strings.NewReader(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %v", resp.Status)
	}

	// invalid HTTP method
	req, _ = http.NewRequest("GET", "http://"+test.svc.BindAddr()+fmt.Sprintf("/users/%s/transfers/id/status", test.userID), nil)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %v", resp.Status)
	}
}

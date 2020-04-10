// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

func TestTransfers__validateUserTransfer(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	xferRepo := &SQLRepo{db.DB, log.NewNopLogger()}

	amt, _ := model.NewAmount("USD", "32.41")
	userID := id.User(base.ID())
	req := &transferRequest{
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}
	transfers, err := xferRepo.createUserTransfers(userID, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Errorf("got %d transfers", len(transfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", fmt.Sprintf("/transfers/%s/failed", transfers[0].ID), nil)
	r.Header.Set("x-user-id", userID.String())

	xferRouter := setupTestRouter(t, xferRepo)
	xferRouter.makeDepository(t, id.Depository(base.ID()))
	xferRouter.makeOriginator(model.OriginatorID(base.ID()))
	xferRouter.makeReceiver(model.ReceiverID(base.ID()))

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d: %s", w.Code, w.Body.String())
	}

	// have our repository error and verify we get non-200's
	mockRepo := &MockRepository{Err: errors.New("bad error")}
	xferRouter.transferRepo = mockRepo

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

// // no repository error, but pretend the ACH file is invalid
// mockRepo.Err = nil
// xferRouter2 := CreateTestTransferRouter(nil, nil, nil, nil, nil, xferRepo)

// router = mux.NewRouter()
// xferRouter2.RegisterRoutes(router)
// w = httptest.NewRecorder()
// router.ServeHTTP(w, r)
// w.Flush()

// if w.Code != http.StatusBadRequest {
// 	t.Errorf("got %d", w.Code)
// }

func TestTransfers__getUserTransferFiles(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	repo := &SQLRepo{db.DB, log.NewNopLogger()}

	amt, _ := model.NewAmount("USD", "32.41")
	userID := id.User(base.ID())
	req := &transferRequest{
		Type:                   model.PushTransfer,
		Amount:                 *amt,
		Originator:             model.OriginatorID("originator"),
		OriginatorDepository:   id.Depository("originator"),
		Receiver:               model.ReceiverID("receiver"),
		ReceiverDepository:     id.Depository("receiver"),
		Description:            "money",
		StandardEntryClassCode: "PPD",
		fileID:                 "test-file",
	}
	transfers, err := repo.createUserTransfers(userID, []*transferRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Errorf("got %d transfers", len(transfers))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", fmt.Sprintf("/transfers/%s/files", transfers[0].ID), nil)
	r.Header.Set("x-user-id", userID.String())

	xferRouter := setupTestRouter(t, repo)
	xferRouter.makeDepository(t, id.Depository(base.ID()))
	xferRouter.makeOriginator(model.OriginatorID(base.ID()))
	xferRouter.makeReceiver(model.ReceiverID(base.ID()))

	router := mux.NewRouter()
	xferRouter.RegisterRoutes(router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}

	bs, _ := ioutil.ReadAll(w.Body)
	bs = bytes.TrimSpace(bs)

	// Verify it's an array returned
	if !bytes.HasPrefix(bs, []byte("[")) || !bytes.HasSuffix(bs, []byte("]")) {
		t.Fatalf("unknown response: %v", string(bs))
	}

	// ach.FileFromJSON doesn't handle multiple files, so for now just break up the array
	file, err := ach.FileFromJSON(bs[1 : len(bs)-1]) // crude strip of [ and ]
	if err != nil || file == nil {
		t.Errorf("file=%v err=%v", file, err)
	}

	// have our repository error and verify we get non-200's
	mockRepo := &MockRepository{Err: errors.New("bad error")}
	xferRouter.transferRepo = mockRepo

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

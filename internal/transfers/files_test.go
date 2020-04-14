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

	"github.com/gorilla/mux"
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

func TestTransfers__validateUserTransfer(t *testing.T) {
	userID := id.User(base.ID())
	amt, _ := model.NewAmount("USD", "32.41")

	transferID := id.Transfer(base.ID())
	repo := &MockRepository{
		Xfer: &model.Transfer{
			ID:                     transferID,
			Type:                   model.PushTransfer,
			Amount:                 *amt,
			Originator:             model.OriginatorID("originator"),
			OriginatorDepository:   id.Depository("originator"),
			Receiver:               model.ReceiverID("receiver"),
			ReceiverDepository:     id.Depository("receiver"),
			Description:            "money",
			StandardEntryClassCode: "PPD",
			PPDDetail: &model.PPDDetail{
				PaymentInformation: "transfer",
			},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", fmt.Sprintf("/transfers/%s/failed", transferID), nil)
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
		t.Errorf("got %d: %s", w.Code, w.Body.String())
	}

	// have our repository error and verify we get non-200's
	repo.Err = errors.New("bad error")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__getUserTransferFiles(t *testing.T) {
	userID := id.User(base.ID())
	amt, _ := model.NewAmount("USD", "32.41")

	transferID := id.Transfer(base.ID())
	repo := &MockRepository{
		Xfer: &model.Transfer{
			ID:                     transferID,
			Type:                   model.PushTransfer,
			Amount:                 *amt,
			Originator:             model.OriginatorID("originator"),
			OriginatorDepository:   id.Depository("originator"),
			Receiver:               model.ReceiverID("receiver"),
			ReceiverDepository:     id.Depository("receiver"),
			Description:            "money",
			StandardEntryClassCode: "PPD",
			PPDDetail: &model.PPDDetail{
				PaymentInformation: "transfer",
			},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", fmt.Sprintf("/transfers/%s/files", transferID), nil)
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
	repo.Err = errors.New("bad error")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

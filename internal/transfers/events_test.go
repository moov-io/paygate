// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/events"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/originators"
	"github.com/moov-io/paygate/internal/receivers"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestTransfers__getUserEvents(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	keeper := secrets.TestStringKeeper(t)

	depRepo := depository.NewDepositoryRepo(log.NewNopLogger(), db.DB, keeper)

	transferID := id.Transfer(base.ID())
	transferRepo := &MockRepository{
		Xfer: &model.Transfer{
			ID: transferID,
		},
	}

	eventRepo := &events.TestRepository{
		Event: &events.Event{
			ID: events.EventID(base.ID()),
		},
	}
	recRepo := &receivers.MockRepository{}
	origRepo := &originators.MockRepository{}

	router := CreateTestTransferRouter(depRepo, eventRepo, nil, recRepo, origRepo, transferRepo)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/transfers/%s/events", transferID), nil)
	r.Header.Set("x-user-id", base.ID())

	router.getUserTransferEvents()(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}
}

func TestTransfers__HTTPGetEventsNoUserID(t *testing.T) {
	xfer := CreateTestTransferRouter(nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()

	xfer.RegisterRoutes(router)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/transfers/foo/events", nil)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
}

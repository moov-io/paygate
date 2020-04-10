// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transfers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/database"
	"github.com/moov-io/paygate/pkg/id"
)

func TestTransfers__getUserEvents(t *testing.T) {
	db := database.CreateTestSqliteDB(t)
	defer db.Close()

	transferID := id.Transfer(base.ID())
	router := setupTestRouter(t, &MockRepository{})

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
	router := setupTestRouter(t, &MockRepository{})
	handler := mux.NewRouter()
	router.RegisterRoutes(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/transfers/foo/events", nil)
	handler.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
}

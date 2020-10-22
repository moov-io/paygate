// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package organization

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/stretchr/testify/require"
)

var (
	orgRepo = &MockRepository{
		Config: &client.OrganizationConfiguration{
			CompanyIdentification: base.ID(),
		},
	}
)

func TestGetOrganizationConfig(t *testing.T) {
	req := httptest.NewRequest("GET", "/configuration/transfers", nil)
	req.Header.Set("X-Organization", "moov")
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	NewRouter(orgRepo).RegisterRoutes(router)
	router.ServeHTTP(w, req)
	w.Flush()

	require.Equal(t, http.StatusOK, w.Code)

	var response client.OrganizationConfiguration
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.CompanyIdentification == "" {
		t.Errorf("CompanyIdentifier=%q", response.CompanyIdentification)
	}
}

func TestUpdateOrganizationConfig(t *testing.T) {
	var body bytes.Buffer
	json.NewEncoder(&body).Encode(&client.OrganizationConfiguration{
		CompanyIdentification: base.ID(),
	})
	req := httptest.NewRequest("PUT", "/configuration/transfers", &body)
	req.Header.Set("X-Organization", "moov")
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	NewRouter(orgRepo).RegisterRoutes(router)
	router.ServeHTTP(w, req)
	w.Flush()

	require.Equal(t, w.Code, http.StatusOK)

	var response client.OrganizationConfiguration
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.CompanyIdentification == "" {
		t.Errorf("CompanyIdentifier=%q", response.CompanyIdentification)
	}
}

func TestUpdateOrganizationConfigOrgNotFound(t *testing.T) {
	orgRepo.Err = errors.New("mock error")
	var body bytes.Buffer
	json.NewEncoder(&body).Encode(&client.OrganizationConfiguration{
		CompanyIdentification: base.ID(),
	})

	req := httptest.NewRequest("PUT", "/configuration/transfers", &body)
	req.Header.Set("X-Organization", "moov")
	req.Header.Set("X-Company", "co123")
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	NewRouter(orgRepo).RegisterRoutes(router)
	router.ServeHTTP(w, req)
	w.Flush()

	require.Equal(t, w.Code, http.StatusBadRequest)
}

func TestUpdateConfigMissingOrganization(t *testing.T) {
	req := httptest.NewRequest("PUT", "/configuration/transfers", nil)
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	NewRouter(orgRepo).RegisterRoutes(router)
	router.ServeHTTP(w, req)
	w.Flush()

	require.Equal(t, w.Code, http.StatusBadRequest)
}

func TestGetConfigErr(t *testing.T) {
	orgRepo.Err = errors.New("mock error")
	req := httptest.NewRequest("GET", "/configuration/transfers", nil)
	req.Header.Set("X-Organization", "moov")
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	NewRouter(orgRepo).RegisterRoutes(router)
	router.ServeHTTP(w, req)
	w.Flush()

	require.Equal(t, w.Code, http.StatusBadRequest)
}

func TestGetConfigMissingOrganization(t *testing.T) {
	req := httptest.NewRequest("GET", "/configuration/transfers", nil)
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	NewRouter(orgRepo).RegisterRoutes(router)
	router.ServeHTTP(w, req)
	w.Flush()

	require.Equal(t, w.Code, http.StatusBadRequest)
}

func TestUpdateConfigMissingCompany(t *testing.T) {
	req := httptest.NewRequest("PUT", "/configuration/transfers", nil)
	req.Header.Set("X-Organization", "moov")
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	NewRouter(orgRepo).RegisterRoutes(router)
	router.ServeHTTP(w, req)
	w.Flush()

	require.Equal(t, w.Code, http.StatusBadRequest)
}

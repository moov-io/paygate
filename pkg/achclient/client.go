// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	"github.com/moov-io/paygate"

	"github.com/go-kit/kit/log"
)

// New creates and returns an ACH instance which can be used to make HTTP requests
// to an ACH service.
//
// There is a shared *http.Client used across all instances.
//
// If ran inside a Kubernetes cluster then Moov's kube-dns record will be the default endpoint.
func New(logger log.Logger, endpoint string, userID string, httpClient *http.Client) *ACH {
	if endpoint == "" {
		if k8s.Inside() {
			endpoint = "http://ach.apps.svc.cluster.local:8080/"
		} else {
			endpoint = "http://localhost" + bind.HTTP("ach")
		}
	}

	logger.Log("ach", fmt.Sprintf("using %s for ACH address", endpoint))

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	return &ACH{
		client:   httpClient,
		endpoint: endpoint,
		logger:   logger,
		userID:   userID,
	}
}

// ACH is an object for interacting with the Moov ACH service.
//
// This is not intended to be a complete implementation of the API endpoints. Moov offers an OpenAPI specification
// and Go client library that does cover the entire set of API endpoints.
type ACH struct {
	client   *http.Client
	endpoint string

	logger log.Logger

	userID string
}

// Ping makes an HTTP GET /ping request to the ACH service and returns any errors encountered.
func (a *ACH) Ping() error {
	resp, err := a.GET("/ping")
	if err != nil {
		return fmt.Errorf("error getting /ping from ACH service: %v", err)
	}
	defer resp.Body.Close()

	// parse content-length header
	n, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return fmt.Errorf("error parsing ACH service /ping response: %v", err)
	}
	if n > 0 {
		return nil
	}
	return fmt.Errorf("no /ping response from ACH")
}

func createRequestId() string {
	bs := make([]byte, 20)
	n, err := rand.Read(bs)
	if err != nil || n == 0 {
		return ""
	}
	return strings.ToLower(hex.EncodeToString(bs))
}

func (a *ACH) addRequestHeaders(idempotencyKey, requestID string, r *http.Request) {
	r.Header.Set("User-Agent", fmt.Sprintf("ach/%s", paygate.Version))
	if idempotencyKey != "" {
		r.Header.Set("X-Idempotency-Key", idempotencyKey)
	}
	if requestID != "" {
		r.Header.Set("X-Request-Id", requestID)
	}
	if a.userID != "" {
		r.Header.Set("X-User-Id", a.userID)
	}
}

// GET performs a HTTP GET request against the a.endpoint and relPath.
// Retries are supported and handled within this method, so if you can't block
// run this method in a goroutine.
func (a *ACH) GET(relPath string) (*http.Response, error) {
	req, err := http.NewRequest("GET", a.buildAddress(relPath), nil)
	if err != nil {
		return nil, err
	}
	requestID := createRequestId()
	a.addRequestHeaders("", requestID, req)
	resp, err := a.client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("ACH GET requestID=%s : %v", requestID, err)
	}
	return resp, nil
}

func (a *ACH) DELETE(relPath string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", a.buildAddress(relPath), nil)
	if err != nil {
		return nil, err
	}
	requestID := createRequestId()
	a.addRequestHeaders("", requestID, req)
	resp, err := a.client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("ACH DELETE requestID=%s : %v", requestID, err)
	}
	return resp, nil
}

// POST performs a HTTP POST request against a.endpoint and relPath.
// Retries are supported only if idempotencyKey is non-empty, otherwise only one attempt is made.
//
// This method assumes a non-nil body is JSON.
func (a *ACH) POST(relPath string, idempotencyKey string, body io.ReadCloser) (*http.Response, error) {
	req, err := http.NewRequest("POST", a.buildAddress(relPath), body)
	if err != nil {
		return nil, err
	}

	requestID := createRequestId()
	a.addRequestHeaders(idempotencyKey, requestID, req)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("ACH POST requestID=%q : %v", requestID, err)
	}
	return resp, nil
}

// buildAddress takes a.endpoint's path and joins it with path to use
// as the full URL for an http.Client request.
//
// This is to handle differences in k8s and local dev. (i.e. /v1/ach/)
func (a *ACH) buildAddress(p string) string {
	u, err := url.Parse(a.endpoint)
	if err != nil {
		return ""
	}
	if u.Scheme == "" && a.logger != nil {
		a.logger.Log("ach", fmt.Sprintf("invalid endpoint=%s", u.String()))
		return ""
	}
	u.Path = path.Join(u.Path, p)
	return u.String()
}

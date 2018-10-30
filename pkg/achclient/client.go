// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/moov-io/paygate/internal/version"

	"github.com/go-kit/kit/log"
)

var (
	// achEndpoint is a DNS record responsible for routing us to an ACH instance.
	// Example: http://ach.apps.svc.cluster.local:8080/
	//
	// If running paygate and ACH with our deployment (i.e. in an apps namespace)
	// you shouldn't need to specify this.
	achEndpoint = os.Getenv("ACH_ENDPOINT")

	achHttpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			MaxConnsPerHost:     100,
			IdleConnTimeout:     1 * time.Minute,
		},
	}

	// kubernetes service account filepath (on default config)
	// https://stackoverflow.com/a/49045575
	k8sServiceAccountFilepath = "/var/run/secrets/kubernetes.io"
)

// New creates and returns an ACH instance which can be used to make HTTP requests
// to an ACH service.
//
// There is a shared *http.Client used across all instances.
//
// If ran inside a Kubernetes cluster then Moov's kube-dns record will be the default endpoint.
func New(userId string, logger log.Logger) *ACH {
	addr := achEndpoint
	if addr == "" {
		if _, err := os.Stat(k8sServiceAccountFilepath); err == nil {
			// We're inside a k8s cluster
			addr = "http://ach.apps.svc.cluster.local:8080/"
		} else {
			// Local development
			addr = "http://localhost:8080/"
		}
	}
	return &ACH{
		client:   achHttpClient,
		endpoint: addr,
		logger:   logger,
		userId:   userId,
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

	userId string
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

func (a *ACH) addRequestHeaders(idempotencyKey, requestId string, r *http.Request) {
	r.Header.Set("User-Agent", fmt.Sprintf("ach/%s", version.Version))
	if idempotencyKey != "" {
		r.Header.Set("X-Idempotency-Key", idempotencyKey)
	}
	if requestId != "" {
		r.Header.Set("X-Request-Id", requestId)
	}
	if a.userId != "" {
		r.Header.Set("X-User-Id", a.userId)
	}
}

// do executes the provided *http.Request with the ACH client.
// Retries are attempted according to a.retryWait
func (a *ACH) do(method string, req *http.Request) (*http.Response, error) {
	return a.client.Do(req)
	// TODO(adam): retries have no body b/c it's read the first time
	// var response *http.Response
	// for n := 1; ; n++ {
	// 	resp, err := a.client.Do(req)
	// 	if err != nil || resp.StatusCode > 499 {
	// 		dur := a.retryWait(n)
	// 		if dur < 0 {
	// 			return response, fmt.Errorf("%s %s after %d attempts: %v", method, req.URL.String(), n, err)
	// 		}
	// 		time.Sleep(dur)
	// 		// TODO(adam): prometheus retry metric ?
	// 		// http_request_retries{target_app="ach", path="${relPath}"}
	// 	} else {
	// 		response = resp
	// 		break
	// 	}
	// }
	// return response, nil
}

// GET performs a HTTP GET request against the a.endpoint and relPath.
// Retries are supported and handled within this method, so if you can't block
// run this method in a goroutine.
func (a *ACH) GET(relPath string) (*http.Response, error) {
	req, err := http.NewRequest("GET", a.buildAddress(relPath), nil)
	if err != nil {
		return nil, err
	}
	requestId := createRequestId()
	a.addRequestHeaders("", requestId, req)
	resp, err := a.do("GET", req)
	if err != nil {
		return resp, fmt.Errorf("ACH GET requestId=%s : %v", requestId, err)
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

	requestId := createRequestId()
	a.addRequestHeaders(idempotencyKey, requestId, req)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// We only will make one HTTP attempt if X-Idempotency-Key is empty.
	// This is done because without a key there's no way to prevent retries, so
	// we've added this to prevent bugs.
	if idempotencyKey == "" {
		resp, err := a.client.Do(req) // call underlying *http.Client
		if err != nil {
			return resp, fmt.Errorf("ACH POST requestId=%q : %v", requestId, err)
		}
		return resp, nil
	}

	// Use our retrying client
	resp, err := a.do("GET", req)
	if err != nil {
		return resp, fmt.Errorf("ACH POST requestId=%q : %v", requestId, err)
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

// retryWait returns the time to wait after n attempts. It works
// off an exponential backoff, but has a max of 125ms.
//
// If the returned duration is negative stop retries.
func (a *ACH) retryWait(n int) time.Duration {
	if n < 0 || n > 4 { // no more than 5 attempts ever
		return -1 * time.Millisecond
	}
	ans := math.Min(math.Pow(2, float64(n))+1, 25) * 5
	return time.Duration(ans) * time.Millisecond
}

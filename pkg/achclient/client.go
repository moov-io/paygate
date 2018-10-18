// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
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

// New creates and returns an ACH instance. This instance can make
func New(requestId string, logger log.Logger) *ACH {
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
		client:    achHttpClient,
		endpoint:  addr,
		logger:    logger,
		requestId: requestId,
	}
}

type ACH struct {
	client   *http.Client
	endpoint string

	logger log.Logger

	requestId string
}

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

func (a *ACH) addRequestHeaders(r *http.Request) {
	r.Header.Set("User-Agent", fmt.Sprintf("ach/%s", version.Version))
	r.Header.Set("X-Request-Id", a.requestId)
}

// GET performs a GET HTTP request against the a.endpoint and relPath.
// Retries are supported and handled within this method, so if you can't block
// run this method in a goroutine.
func (a *ACH) GET(relPath string) (*http.Response, error) {
	req, err := http.NewRequest("GET", a.buildAddress(relPath), nil)
	if err != nil {
		return nil, err
	}
	a.addRequestHeaders(req)

	var response *http.Response
	for n := 0; ; n++ {
		resp, err := a.client.Do(req)
		if err != nil {
			dur := a.retryWait(n)
			if dur < 0 {
				return response, fmt.Errorf("GET %s after %d attempts: %v", relPath, n, err)
			}

			t := time.NewTimer(dur)
			select {
			case <-t.C:
				// wait
			}

			// TODO(adam): prometheus retry metric ?
			// http_request_retries{target_app="ach", path="${relPath}"}
		} else {
			response = resp
			break
		}
	}
	return response, nil
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
	if u.Scheme == "" {
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

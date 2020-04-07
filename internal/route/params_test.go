// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package route

import (
	"net/http"
	"net/url"
	"testing"
)

func TestReadLimit(t *testing.T) {
	req := makeRequest(t, "http://localhost:8082/transfers?limit=27")
	if limit := ReadLimit(req); limit != 27 {
		t.Errorf("limit=%d", limit)
	}

	req = makeRequest(t, "http://localhost:8082/transfers?limit=2700")
	if limit := ReadLimit(req); limit != 1000 {
		t.Errorf("limit=%d", limit)
	}

	req = makeRequest(t, "http://localhost:8082/transfers")
	if limit := ReadLimit(req); limit != 0 {
		t.Errorf("limit=%d", limit)
	}
}

func TestReadOffset(t *testing.T) {
	req := makeRequest(t, "http://localhost:8082/transfers?offset=27")
	if offset := ReadOffset(req); offset != 27 {
		t.Errorf("offset=%d", offset)
	}

	req = makeRequest(t, "http://localhost:8082/transfers")
	if offset := ReadOffset(req); offset != 0 {
		t.Errorf("offset=%d", offset)
	}
}

func makeRequest(t *testing.T, in string) *http.Request {
	u, _ := url.Parse(in)
	return &http.Request{
		URL: u,
	}
}

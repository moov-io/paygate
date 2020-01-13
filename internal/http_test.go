// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

func TestRead(t *testing.T) {
	r := Read(strings.NewReader("bar baz"))
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if s := string(bs); s != "bar baz" {
		t.Errorf("got %s", s)
	}

	r = Read(strings.NewReader(strings.Repeat("a", maxReadBytes+100)))
	bs, err = ioutil.ReadAll(r)
	if n := len(bs); n != maxReadBytes || err != nil {
		t.Errorf("length=%d error=%v", n, err)
	}
}

func TestHttp__AddPingRoute(t *testing.T) {
	r := httptest.NewRequest("GET", "/ping", nil)
	r.Header.Set("x-request-id", base.ID())
	w := httptest.NewRecorder()

	router := mux.NewRouter()
	AddPingRoute(log.NewNopLogger(), router)
	router.ServeHTTP(w, r)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}
	if v := w.Body.String(); v != "PONG" {
		t.Errorf("got %q", v)
	}
}

func TestHTTP__TLSHttpClient(t *testing.T) {
	client, err := TLSHttpClient("")
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Error("empty http.Client")
	}

	if testing.Short() {
		return // skip network call on -short
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", "google.com:443", nil)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	fd, err := ioutil.TempFile("", "TLSHttpClient")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(fd.Name())

	// Write x509 certs to disk
	certs := conn.ConnectionState().PeerCertificates
	var buf bytes.Buffer
	for i := range certs {
		b := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certs[i].Raw,
		}
		if err := pem.Encode(&buf, b); err != nil {
			t.Fatal(err)
		}
	}
	if err := ioutil.WriteFile(fd.Name(), buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	client, err = TLSHttpClient(fd.Name())
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Error("empty http.Client")
	}
}

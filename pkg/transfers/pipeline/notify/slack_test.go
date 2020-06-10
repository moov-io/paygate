// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moov-io/paygate/pkg/config"

	"github.com/gorilla/mux"
)

func TestSlack(t *testing.T) {
	handler := mux.NewRouter()
	handler.Methods("POST").Path("/webhook").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bs, _ := ioutil.ReadAll(r.Body)
		if bytes.Contains(bs, []byte(`"text"`)) {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	svc := httptest.NewServer(handler)
	defer svc.Close()

	cfg := &config.Slack{
		WebhookURL: svc.URL + "/webhook",
	}
	slack, err := NewSlack(cfg)
	if err != nil {
		t.Fatal(err)
	}

	msg := &Message{
		Direction: Download,
		Filename:  "20200529-152259.ach",
	}

	if err := slack.Info(msg); err != nil {
		t.Fatal(err)
	}

	if err := slack.Critical(msg); err != nil {
		t.Fatal(err)
	}
}

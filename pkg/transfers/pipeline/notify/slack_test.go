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

	"github.com/stretchr/testify/require"

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

func TestSlack__marshal(t *testing.T) {
	tests := []struct {
		desc          string
		status        uploadStatus
		msg           *Message
		shouldContain string
	}{
		{"successful upload with hostname", success, &Message{Direction: Upload, Filename: "myfile.txt", Hostname: "ftp.mybank.com:1234"},
			"SUCCESSFUL upload of myfile.txt to ftp.mybank.com:1234"},
		{"failed upload with hostname", failed, &Message{Direction: Upload, Filename: "myfile.txt", Hostname: "ftp.mybank.com:1234"},
			"FAILED upload of myfile.txt to ftp.mybank.com:1234"},
		{"successful download", success, &Message{Direction: Download, Filename: "myfile.txt", Hostname: "ftp.mybank.com:1234"},
			"SUCCESSFUL download of myfile.txt from ftp.mybank.com:1234 with ODFI server"},
		{"failed download", failed, &Message{Direction: Download, Filename: "myfile.txt", Hostname: "ftp.mybank.com:1234"},
			"FAILED download of myfile.txt from ftp.mybank.com:1234 with ODFI server"},
	}

	for _, test := range tests {
		actual := marshalSlackMessage(test.status, test.msg)
		require.Contains(t, actual, test.shouldContain)
	}
}
